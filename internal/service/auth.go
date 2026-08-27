package service

import (
	"context"
	"errors"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/auth"
	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

// UserStore/TokenStore are the persistence seams AuthService depends on.
type UserStore interface {
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	Create(ctx context.Context, u *model.User) error
}

type TokenStore interface {
	Create(ctx context.Context, t *model.PersonalAccessToken) error
	RevokeByTokenHash(ctx context.Context, tokenHash string) error
}

// EmailUnverifiedError is not an HTTP error itself: it is a domain flow result
// (Laravel login returns 403 when email_verified_at is null). The handler maps
// it to the 403 body carrying the partial user.
type EmailUnverifiedError struct{ User *model.User }

func (e EmailUnverifiedError) Error() string { return "email not verified" }

// InvalidCredentialsError is the generic 401 outcome. It deliberately carries
// no hint whether email or password was wrong (anti user-enumeration).
type InvalidCredentialsError struct{}

func (InvalidCredentialsError) Error() string { return "invalid credentials" }

// AuthService orchestrates register/login/revoke. No HTTP knowledge.
type AuthService struct {
	users     UserStore
	tokens    TokenStore
	hasher    auth.PasswordHasher
	generator auth.TokenGenerator
}

func NewAuthService(users UserStore, tokens TokenStore, hasher auth.PasswordHasher, generator auth.TokenGenerator) *AuthService {
	return &AuthService{users: users, tokens: tokens, hasher: hasher, generator: generator}
}

type RegisterInput struct {
	Name     string
	Email    string
	Password string // plaintext from request; never persisted
}

// Register hashes the password (concern here, not repository: hashing is
// application policy that must apply to every write path) and persists the
// user. Laravel mirrors this: the controller delegates hashing to the model's
// "password" cast, not to SQL. Duplicate email -> typed validation.
//
// Laravel's OTP email side effect is NOT implemented in this phase
// (deferred: FASE 7B-3) — documented in docs/authentication-core.md.
func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*model.User, error) {
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, httperr.Internal(err)
	}

	u := &model.User{
		Name:     in.Name,
		Email:    in.Email,
		Password: hash,
		Role:     model.RoleCustomer,
	}
	if err := s.users.Create(ctx, u); err != nil {
		if errors.Is(err, repository.ErrDuplicateEmail) {
			return nil, httperr.Validation(map[string][]string{
				"email": {"The email has already been taken."},
			})
		}
		return nil, httperr.Internal(err)
	}
	return u, nil
}

// LoginResult is what a successful (verified) login yields.
type LoginResult struct {
	User     *model.User
	RawToken string
}

// Login verifies credentials exactly like Laravel:
//   - unknown user OR wrong password  -> InvalidCredentialsError (generic 401)
//   - unverified email                -> EmailUnverifiedError (403 with user)
//   - otherwise                       -> token row stored (hash), raw returned
func (s *AuthService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	u, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, InvalidCredentialsError{}
		}
		return nil, httperr.Internal(err)
	}

	if err := s.hasher.Compare(u.Password, password); err != nil {
		return nil, InvalidCredentialsError{}
	}

	if u.EmailVerifiedAt == nil {
		return nil, EmailUnverifiedError{User: u}
	}

	raw, hash, err := s.generator.Generate()
	if err != nil {
		return nil, httperr.Internal(err)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(model.DefaultTokenLifetime).UTC()
	token := &model.PersonalAccessToken{
		TokenableType: model.TokenableType,
		TokenableID:   u.ID,
		Name:          "mobile-app",
		Token:         hash,
		ExpiresAt:     &expiresAt,
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := s.tokens.Create(ctx, token); err != nil {
		return nil, httperr.Internal(err)
	}

	return &LoginResult{User: u, RawToken: raw}, nil
}

// RevokeToken deletes the token row matching the raw bearer token hash.
// Missing token is not an error (Laravel logout still returns success).
func (s *AuthService) RevokeToken(ctx context.Context, rawToken string) error {
	return s.tokens.RevokeByTokenHash(ctx, s.generator.Hash(rawToken))
}
