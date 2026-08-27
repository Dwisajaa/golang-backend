package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/auth"
	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type fakeUserStore struct {
	byEmail map[string]*model.User
	created []*model.User
	err     error
}

func (f *fakeUserStore) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, repository.ErrNotFound
}

func (f *fakeUserStore) Create(ctx context.Context, u *model.User) error {
	if f.err != nil {
		return f.err
	}
	if _, ok := f.byEmail[u.Email]; ok {
		return repository.ErrDuplicateEmail
	}
	u.ID = uint64(len(f.created) + 1)
	f.byEmail[u.Email] = u
	f.created = append(f.created, u)
	return nil
}

type fakeTokenStore struct {
	rows    []*model.PersonalAccessToken
	created []*model.PersonalAccessToken
	revoked []string
	err     error
}

func (f *fakeTokenStore) Create(ctx context.Context, t *model.PersonalAccessToken) error {
	if f.err != nil {
		return f.err
	}
	t.ID = uint64(len(f.rows) + 1)
	f.rows = append(f.rows, t)
	f.created = append(f.created, t)
	return nil
}

func (f *fakeTokenStore) RevokeByTokenHash(ctx context.Context, hash string) error {
	if f.err != nil {
		return f.err
	}
	f.revoked = append(f.revoked, hash)
	return nil
}

func newFakeStores() (*fakeUserStore, *fakeTokenStore) {
	return &fakeUserStore{byEmail: map[string]*model.User{}}, &fakeTokenStore{}
}

func newAuth() (*AuthService, *fakeUserStore, *fakeTokenStore) {
	us, ts := newFakeStores()
	svc := NewAuthService(us, ts, auth.NewBcryptHasher(), auth.NewRandomTokenGenerator())
	return svc, us, ts
}

func TestRegisterStoresHashNotPlaintext(t *testing.T) {
	svc, us, _ := newAuth()
	u, err := svc.Register(context.Background(), RegisterInput{
		Name: "A", Email: "a@example.test", Password: "password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if u.Role != model.RoleCustomer {
		t.Fatalf("role should default to customer, got %q", u.Role)
	}
	stored := us.byEmail["a@example.test"]
	if stored.Password == "password123" {
		t.Fatal("password stored as plaintext")
	}
	if stored.Password == "" {
		t.Fatal("password hash missing")
	}
	// hash must verify the original password (bcrypt)
	hasher := auth.NewBcryptHasher()
	if err := hasher.Compare(stored.Password, "password123"); err != nil {
		t.Fatalf("stored hash does not verify: %v", err)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	svc, us, _ := newAuth()
	us.byEmail["dup@example.test"] = &model.User{Email: "dup@example.test"}
	_, err := svc.Register(context.Background(), RegisterInput{Name: "B", Email: "dup@example.test", Password: "password123"})
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
	if len(he.Errors["email"]) == 0 {
		t.Fatal("expected email error detail")
	}
}

func TestRegisterRepositoryFailure(t *testing.T) {
	svc, us, _ := newAuth()
	us.err = errors.New("db down")
	_, err := svc.Register(context.Background(), RegisterInput{Name: "C", Email: "c@example.test", Password: "password123"})
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindInternal {
		t.Fatalf("expected internal error, got %v", err)
	}
}

func TestLoginSuccessReturnsRawTokenAndStoresHash(t *testing.T) {
	svc, us, ts := newAuth()
	hasher := auth.NewBcryptHasher()
	hash, _ := hasher.Hash("password123")
	us.byEmail["v@example.test"] = &model.User{
		ID: 9, Name: "V", Email: "v@example.test", Password: hash, Role: model.RoleCustomer,
		EmailVerifiedAt: pt(),
	}

	res, err := svc.Login(context.Background(), "v@example.test", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.RawToken == "" {
		t.Fatal("raw token missing")
	}
	if len(ts.created) != 1 {
		t.Fatalf("expected 1 token row, got %d", len(ts.created))
	}
	stored := ts.created[0]
	if stored.Token == res.RawToken {
		t.Fatal("stored token must be the hash, not the raw token")
	}
	gen := auth.NewRandomTokenGenerator()
	if gen.Hash(res.RawToken) != stored.Token {
		t.Fatal("stored token must equal sha256(raw)")
	}
	if stored.TokenableType != model.TokenableType || stored.TokenableID != 9 || stored.Name != "mobile-app" {
		t.Fatalf("unexpected token row: %+v", stored)
	}
	if stored.ExpiresAt == nil {
		t.Fatal("token must have expiration")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc, us, _ := newAuth()
	hash, _ := auth.NewBcryptHasher().Hash("right")
	us.byEmail["w@example.test"] = &model.User{
		ID: 1, Email: "w@example.test", Password: hash, Role: model.RoleCustomer, EmailVerifiedAt: pt(),
	}
	_, err := svc.Login(context.Background(), "w@example.test", "wrong")
	var inv InvalidCredentialsError
	if !errors.As(err, &inv) {
		t.Fatalf("expected InvalidCredentialsError, got %v", err)
	}
}

func TestLoginUnknownUserIsGeneric(t *testing.T) {
	svc, us, _ := newAuth()
	// seed a real user so lookup EXISTS in the store — we login as a stranger
	hash, _ := auth.NewBcryptHasher().Hash("x")
	us.byEmail["real@example.test"] = &model.User{ID: 1, Email: "real@example.test", Password: hash, Role: model.RoleCustomer, EmailVerifiedAt: pt()}
	_, err := svc.Login(context.Background(), "nobody@example.test", "anything")
	var inv InvalidCredentialsError
	if !errors.As(err, &inv) {
		t.Fatalf("unknown user must map to the same generic error, got %v", err)
	}
}

func TestLoginUnverifiedEmail(t *testing.T) {
	svc, us, _ := newAuth()
	hash, _ := auth.NewBcryptHasher().Hash("password123")
	us.byEmail["unv@example.test"] = &model.User{ID: 2, Name: "U", Email: "unv@example.test", Password: hash, Role: model.RoleCustomer}

	_, err := svc.Login(context.Background(), "unv@example.test", "password123")
	var unv EmailUnverifiedError
	if !errors.As(err, &unv) {
		t.Fatalf("expected EmailUnverifiedError, got %v", err)
	}
	if unv.User.Email != "unv@example.test" {
		t.Fatalf("unverified error should carry the user, got %+v", unv.User)
	}
}

func TestLoginTokenPersistenceError(t *testing.T) {
	svc, us, ts := newAuth()
	hash, _ := auth.NewBcryptHasher().Hash("password123")
	us.byEmail["v@example.test"] = &model.User{ID: 9, Email: "v@example.test", Password: hash, Role: model.RoleCustomer, EmailVerifiedAt: pt()}
	ts.err = errors.New("insert failed")
	_, err := svc.Login(context.Background(), "v@example.test", "password123")
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindInternal {
		t.Fatalf("expected internal on token persist failure, got %v", err)
	}
}

func TestRevokeTokenHashesAndRevokes(t *testing.T) {
	svc, _, ts := newAuth()
	raw := "some-raw-token"
	if err := svc.RevokeToken(context.Background(), raw); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	want := auth.NewRandomTokenGenerator().Hash(raw)
	if len(ts.revoked) != 1 || ts.revoked[0] != want {
		t.Fatalf("expected revoke of hash %q, got %v", want, ts.revoked)
	}
}

func pt() *time.Time {
	t := time.Now().UTC()
	return &t
}
