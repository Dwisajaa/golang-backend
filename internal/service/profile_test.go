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

type fakeProfileUsers struct {
	byID            map[uint64]*model.User
	byEmail         map[string]*model.User
	savedName       string
	savedEmail      string
	savedVerifiedAt *time.Time
	savedPwdHash    string
	err             error
}

func (f *fakeProfileUsers) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	u, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (f *fakeProfileUsers) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, repository.ErrNotFound
}

func (f *fakeProfileUsers) UpdateNameEmail(ctx context.Context, q repository.Queryer, id uint64, name, email string, verifiedAt *time.Time) error {
	if f.err != nil {
		return f.err
	}
	if other, ok := f.byEmail[email]; ok && other.ID != id {
		return repository.ErrDuplicateEmail
	}
	f.savedName = name
	f.savedEmail = email
	f.savedVerifiedAt = verifiedAt
	return nil
}

func (f *fakeProfileUsers) UpdatePassword(ctx context.Context, q repository.Queryer, id uint64, passwordHash string) error {
	if f.err != nil {
		return f.err
	}
	f.savedPwdHash = passwordHash
	return nil
}

type fakeTokenRevo struct {
	revoked []uint64
	err     error
}

func (f *fakeTokenRevo) RevokeAllForUser(ctx context.Context, q repository.Queryer, userID uint64) error {
	if f.err != nil {
		return f.err
	}
	f.revoked = append(f.revoked, userID)
	return nil
}

type fakeOtpDispatcher struct {
	called []string
	err    error
}

func (f *fakeOtpDispatcher) ResendVerificationOtp(ctx context.Context, email string) error {
	if f.err != nil {
		return f.err
	}
	f.called = append(f.called, email)
	return nil
}

func newProfileSvc(t *testing.T, users *fakeProfileUsers, tokens *fakeTokenRevo, otp *fakeOtpDispatcher) *ProfileService {
	t.Helper()
	return NewProfileService(users, tokens, fakeTx{}, auth.NewBcryptHasher(), otp)
}

func (f *fakeProfileUsers) seed(id uint64, email string, verified bool) *model.User {
	u := &model.User{ID: id, Name: "U", Email: email, Role: model.RoleCustomer, Password: hashForTest()}
	if verified {
		now := time.Now().UTC()
		u.EmailVerifiedAt = &now
	}
	f.byID[id] = u
	f.byEmail[email] = u
	return u
}

func hashForTest() string {
	h, _ := auth.NewBcryptHasher().Hash("password123")
	return h
}

func TestUpdateProfileEmailUnchangedKeepsVerified(t *testing.T) {
	users := &fakeProfileUsers{byID: map[uint64]*model.User{}, byEmail: map[string]*model.User{}}
	users.seed(1, "u@example.test", true)
	otp := &fakeOtpDispatcher{}
	svc := newProfileSvc(t, users, &fakeTokenRevo{}, otp)

	u, err := svc.UpdateProfile(context.Background(), 1, UpdateProfileInput{Name: "New", Email: "u@example.test"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if users.savedVerifiedAt == nil {
		t.Fatal("verified timestamp must be preserved when email is unchanged")
	}
	if u.EmailVerifiedAt == nil {
		t.Fatal("user must stay verified")
	}
	if len(otp.called) != 0 {
		t.Fatal("no OTP expected when email unchanged")
	}
}

func TestUpdateProfileEmailChangedResetsVerifiedAndDispatchesOtp(t *testing.T) {
	users := &fakeProfileUsers{byID: map[uint64]*model.User{}, byEmail: map[string]*model.User{}}
	users.seed(1, "old@example.test", true)
	otp := &fakeOtpDispatcher{}
	svc := newProfileSvc(t, users, &fakeTokenRevo{}, otp)

	u, err := svc.UpdateProfile(context.Background(), 1, UpdateProfileInput{Name: "New", Email: "new@example.test"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if users.savedVerifiedAt != nil {
		t.Fatal("verifiedAt must be NULL when email changes (Laravel reset)")
	}
	if u.EmailVerifiedAt != nil {
		t.Fatal("email_verified_at must be nil after email change")
	}
	if len(otp.called) != 1 || otp.called[0] != "new@example.test" {
		t.Fatalf("expected OTP dispatch for new email, got %v", otp.called)
	}
}

func TestUpdateProfileDuplicateEmail(t *testing.T) {
	users := &fakeProfileUsers{byID: map[uint64]*model.User{}, byEmail: map[string]*model.User{}}
	users.seed(1, "me@example.test", true)
	users.seed(2, "other@example.test", true)
	otp := &fakeOtpDispatcher{}
	svc := newProfileSvc(t, users, &fakeTokenRevo{}, otp)

	_, err := svc.UpdateProfile(context.Background(), 1, UpdateProfileInput{Name: "A", Email: "other@example.test"})
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindValidation {
		t.Fatalf("expected 422 duplicate email, got %v", err)
	}
}

func TestUpdateProfileRepositoryError(t *testing.T) {
	users := &fakeProfileUsers{byID: map[uint64]*model.User{}, byEmail: map[string]*model.User{}}
	users.seed(1, "u@example.test", false)
	users.err = errors.New("db down")
	svc := newProfileSvc(t, users, &fakeTokenRevo{}, &fakeOtpDispatcher{})

	_, err := svc.UpdateProfile(context.Background(), 1, UpdateProfileInput{Name: "A", Email: "u@example.test"})
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindInternal {
		t.Fatalf("expected 500, got %v", err)
	}
}

func TestUpdatePasswordWrongCurrent(t *testing.T) {
	users := &fakeProfileUsers{byID: map[uint64]*model.User{}, byEmail: map[string]*model.User{}}
	users.seed(1, "u@example.test", true)
	tokens := &fakeTokenRevo{}
	svc := newProfileSvc(t, users, tokens, &fakeOtpDispatcher{})

	err := svc.UpdatePassword(context.Background(), 1, "wrong-current", "newpassword123")
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindValidation {
		t.Fatalf("expected 422 wrong current password, got %v", err)
	}
	if len(he.Errors["current_password"]) == 0 {
		t.Fatalf("expected current_password error detail: %v", he.Errors)
	}
	if len(tokens.revoked) != 0 {
		t.Fatal("tokens must not be revoked on failed password change")
	}
}

func TestUpdatePasswordSuccessHashesAndRevokes(t *testing.T) {
	users := &fakeProfileUsers{byID: map[uint64]*model.User{}, byEmail: map[string]*model.User{}}
	users.seed(1, "u@example.test", true)
	tokens := &fakeTokenRevo{}
	svc := newProfileSvc(t, users, tokens, &fakeOtpDispatcher{})

	if err := svc.UpdatePassword(context.Background(), 1, "password123", "newpassword123"); err != nil {
		t.Fatalf("update password: %v", err)
	}
	if users.savedPwdHash == "" || users.savedPwdHash == "newpassword123" {
		t.Fatal("password must be stored hashed")
	}
	hasher := auth.NewBcryptHasher()
	if err := hasher.Compare(users.savedPwdHash, "newpassword123"); err != nil {
		t.Fatalf("stored hash must verify new password: %v", err)
	}
	if len(tokens.revoked) != 1 || tokens.revoked[0] != 1 {
		t.Fatalf("expected all tokens revoked for user 1, got %v", tokens.revoked)
	}
}

func TestUpdatePasswordUserNotFound(t *testing.T) {
	users := &fakeProfileUsers{byID: map[uint64]*model.User{}, byEmail: map[string]*model.User{}}
	svc := newProfileSvc(t, users, &fakeTokenRevo{}, &fakeOtpDispatcher{})

	err := svc.UpdatePassword(context.Background(), 99, "x", "newpassword123")
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}
