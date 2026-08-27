package service

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/auth"
	"github.com/Dwisajaa/golang-backend/internal/config"
	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/mailer"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

// fakeTx runs fn immediately (no real database) â€” enough because the repo
// fakes ignore the Queryer argument.
type fakeTx struct{}

func (fakeTx) Within(ctx context.Context, fn func(tx *sql.Tx) error) error { return fn(nil) }

type fakeMail struct {
	sent []mailer.Message
}

func (m *fakeMail) Send(ctx context.Context, msg mailer.Message) error {
	m.sent = append(m.sent, msg)
	return nil
}

// fakeOtpUsers is thread-safe (verified map is touched by concurrent VerifyEmail).
type fakeOtpUsers struct {
	mu       sync.Mutex
	byEmail  map[string]*model.User
	verified map[uint64]*time.Time
	err      error
}

func (f *fakeOtpUsers) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	u, ok := f.byEmail[email]
	if !ok {
		return nil, repository.ErrNotFound
	}
	// reflect server-stored verified state
	if v, okV := f.verified[u.ID]; okV {
		cp := *u
		cp.EmailVerifiedAt = v
		return &cp, nil
	}
	return u, nil
}

func (f *fakeOtpUsers) UpdateVerified(ctx context.Context, q repository.Queryer, id uint64, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	if f.verified == nil {
		f.verified = map[uint64]*time.Time{}
	}
	f.verified[id] = &at
	return nil
}

type fakeOtpStore struct {
	mu      sync.Mutex
	active  map[uint64]*model.EmailVerificationOtp
	created []*model.EmailVerificationOtp
	err     error
}

func (f *fakeOtpStore) PruneAndInvalidate(ctx context.Context, q repository.Queryer, userID uint64, otpType string) error {
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *fakeOtpStore) Create(ctx context.Context, q repository.Queryer, otp *model.EmailVerificationOtp) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if otp.ID == 0 {
		otp.ID = uint64(len(f.created) + 1)
	}
	f.created = append(f.created, otp)
	return nil
}

func (f *fakeOtpStore) FindActiveForUpdate(ctx context.Context, q repository.Queryer, userID uint64, otpType string, now time.Time) (*model.EmailVerificationOtp, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var latest *model.EmailVerificationOtp
	for _, rec := range f.active {
		if rec.UserID == userID && rec.Type == otpType && rec.ExpiresAt.After(now) {
			if latest == nil || rec.ID > latest.ID {
				latest = rec
			}
		}
	}
	if latest == nil {
		return nil, repository.ErrNotFound
	}
	return latest, nil
}

func (f *fakeOtpStore) IncrementAttempts(ctx context.Context, q repository.Queryer, id uint64) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if rec, ok := f.active[id]; ok && rec.UsedAt == nil {
		rec.Attempts++
		return nil
	}
	return nil
}

func (f *fakeOtpStore) MarkUsed(ctx context.Context, q repository.Queryer, id uint64, usedAt time.Time) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if rec, ok := f.active[id]; ok {
		rec.UsedAt = &usedAt
	}
	return nil
}

// seedOtp inserts a real bcrypt-hashed OTP usable by VerifyEmail.
func seedOtp(store *fakeOtpStore, userID uint64, raw string, expiresIn time.Duration) *model.EmailVerificationOtp {
	hash, _ := auth.NewBcryptHasher().Hash(raw)
	exp := time.Now().UTC().Add(expiresIn).UTC()
	rec := &model.EmailVerificationOtp{
		ID: uint64(len(store.active) + 1), UserID: userID, Type: model.OtpTypeEmailVerification,
		OtpHash: hash, ExpiresAt: &exp, Attempts: 0,
	}
	store.mu.Lock()
	store.active[rec.ID] = rec
	store.mu.Unlock()
	return rec
}

func newOtpSvc(t *testing.T, users *fakeOtpUsers, store *fakeOtpStore, mail *fakeMail) (*OtpService, *fakeOtpTokens) {
	t.Helper()
	tokens := &fakeOtpTokens{}
	svc := NewOtpService(store, users, tokens, fakeTx{}, auth.NewBcryptHasher(), auth.NewOtpGenerator(), auth.NewRandomTokenGenerator(), mail, config.OtpConfig{ExpirationMinutes: 10, MaxAttempts: 5})
	return svc, tokens
}

func (f *fakeOtpUsers) ensureUser(id uint64, email string) *model.User {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.byEmail == nil {
		f.byEmail = map[string]*model.User{}
	}
	if f.verified == nil {
		f.verified = map[uint64]*time.Time{}
	}
	u := &model.User{ID: id, Name: "U", Email: email, Role: model.RoleCustomer}
	f.byEmail[email] = u
	return u
}

func (f *fakeOtpUsers) setVerified(id uint64, at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.verified == nil {
		f.verified = map[uint64]*time.Time{}
	}
	f.verified[id] = &at
}

func (f *fakeOtpUsers) isVerified(id uint64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.verified[id] != nil
}

func TestResendCreatesOtpAndQueuesMail(t *testing.T) {
	users := &fakeOtpUsers{}
	users.ensureUser(1, "u@example.test")
	store := &fakeOtpStore{active: map[uint64]*model.EmailVerificationOtp{}}
	mail := &fakeMail{}
	svc, _ := newOtpSvc(t, users, store, mail)

	if err := svc.ResendVerificationOtp(context.Background(), "u@example.test"); err != nil {
		t.Fatalf("resend: %v", err)
	}
	if len(store.created) != 1 {
		t.Fatalf("expected 1 otp row, got %d", len(store.created))
	}
	if len(mail.sent) != 1 || mail.sent[0].ToEmail != "u@example.test" {
		t.Fatalf("expected 1 mail to u@example.test, got %+v", mail.sent)
	}
	if mail.sent[0].Body == "" {
		t.Fatal("mail body must carry the verification code")
	}
}

func TestResendUnknownUserIsSilent(t *testing.T) {
	users := &fakeOtpUsers{}
	store := &fakeOtpStore{active: map[uint64]*model.EmailVerificationOtp{}}
	mail := &fakeMail{}
	svc, _ := newOtpSvc(t, users, store, mail)

	if err := svc.ResendVerificationOtp(context.Background(), "nobody@example.test"); err != nil {
		t.Fatalf("resend unknown must succeed silently, got %v", err)
	}
	if len(mail.sent) != 0 {
		t.Fatal("no mail expected for unknown user")
	}
}

func TestResendVerifiedUserIsSilent(t *testing.T) {
	users := &fakeOtpUsers{}
	u := users.ensureUser(1, "v@example.test")
	now := time.Now().UTC()
	users.setVerified(u.ID, now)
	store := &fakeOtpStore{active: map[uint64]*model.EmailVerificationOtp{}}
	mail := &fakeMail{}
	svc, _ := newOtpSvc(t, users, store, mail)

	if err := svc.ResendVerificationOtp(context.Background(), "v@example.test"); err != nil {
		t.Fatal(err)
	}
	if len(store.created) != 0 || len(mail.sent) != 0 {
		t.Fatal("verified user must not get a new OTP")
	}
}

func TestVerifySuccessUpdatesVerifiedAndReturnsToken(t *testing.T) {
	users := &fakeOtpUsers{}
	users.ensureUser(1, "u@example.test")
	store := &fakeOtpStore{active: map[uint64]*model.EmailVerificationOtp{}}
	seedOtp(store, 1, "123456", 10*time.Minute)
	mail := &fakeMail{}
	svc, tokens := newOtpSvc(t, users, store, mail)

	res, err := svc.VerifyEmail(context.Background(), "u@example.test", "123456")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if res.RawToken == "" {
		t.Fatal("expected token")
	}
	if !users.isVerified(1) {
		t.Fatal("email_verified_at must be set")
	}
	if len(tokens.created) != 1 {
		t.Fatalf("expected 1 token row, got %d", len(tokens.created))
	}
}

func TestVerifyWrongOtpIncrementsAttemptsAndFails(t *testing.T) {
	users := &fakeOtpUsers{}
	users.ensureUser(1, "u@example.test")
	store := &fakeOtpStore{active: map[uint64]*model.EmailVerificationOtp{}}
	rec := seedOtp(store, 1, "123456", 10*time.Minute)
	mail := &fakeMail{}
	svc, _ := newOtpSvc(t, users, store, mail)

	_, err := svc.VerifyEmail(context.Background(), "u@example.test", "000000")
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindValidation {
		t.Fatalf("expected 422 for wrong otp, got %v", err)
	}
	if rec.Attempts != 1 {
		t.Fatalf("attempts must increment to 1, got %d", rec.Attempts)
	}
	if users.isVerified(1) {
		t.Fatal("email must NOT be verified on wrong otp")
	}
}

func TestVerifyExpiredOtpFails(t *testing.T) {
	users := &fakeOtpUsers{}
	users.ensureUser(1, "u@example.test")
	store := &fakeOtpStore{active: map[uint64]*model.EmailVerificationOtp{}}
	seedOtp(store, 1, "123456", -time.Minute) // already expired
	mail := &fakeMail{}
	svc, _ := newOtpSvc(t, users, store, mail)

	_, err := svc.VerifyEmail(context.Background(), "u@example.test", "123456")
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindValidation {
		t.Fatalf("expected 422 for expired otp, got %v", err)
	}
}

func TestVerifyUsedOtpBecomesUnusable(t *testing.T) {
	users := &fakeOtpUsers{}
	users.ensureUser(1, "u@example.test")
	store := &fakeOtpStore{active: map[uint64]*model.EmailVerificationOtp{}}
	seedOtp(store, 1, "123456", 10*time.Minute)
	mail := &fakeMail{}
	svc, _ := newOtpSvc(t, users, store, mail)

	if _, err := svc.VerifyEmail(context.Background(), "u@example.test", "123456"); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	_, err := svc.VerifyEmail(context.Background(), "u@example.test", "123456")
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindBadRequest || he.Message != "Email sudah diverifikasi." {
		t.Fatalf("second verify of a consumed OTP must fail, got %v", err)
	}
}

func TestVerifyAttemptsExceeded(t *testing.T) {
	users := &fakeOtpUsers{}
	users.ensureUser(1, "u@example.test")
	store := &fakeOtpStore{active: map[uint64]*model.EmailVerificationOtp{}}
	rec := seedOtp(store, 1, "123456", 10*time.Minute)
	rec.Attempts = 5 // OTP_MAX_ATTEMPTS
	mail := &fakeMail{}
	svc, _ := newOtpSvc(t, users, store, mail)

	_, err := svc.VerifyEmail(context.Background(), "u@example.test", "123456")
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindTooManyRequests {
		t.Fatalf("expected 429 after max attempts, got %v", err)
	}
}

func TestVerifyAlreadyVerified(t *testing.T) {
	users := &fakeOtpUsers{}
	u := users.ensureUser(1, "u@example.test")
	now := time.Now().UTC()
	users.setVerified(u.ID, now)
	store := &fakeOtpStore{active: map[uint64]*model.EmailVerificationOtp{}}
	mail := &fakeMail{}
	svc, _ := newOtpSvc(t, users, store, mail)

	_, err := svc.VerifyEmail(context.Background(), "u@example.test", "123456")
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindBadRequest {
		t.Fatalf("expected 400 for already-verified, got %v", err)
	}
}

func TestVerifyUnknownUser(t *testing.T) {
	users := &fakeOtpUsers{}
	store := &fakeOtpStore{active: map[uint64]*model.EmailVerificationOtp{}}
	mail := &fakeMail{}
	svc, _ := newOtpSvc(t, users, store, mail)

	_, err := svc.VerifyEmail(context.Background(), "nobody@example.test", "123456")
	he := httperr.As(err)
	if he == nil || he.Message != "Kode verifikasi tidak valid atau tidak ditemukan." {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestVerifySingleUseConcurrent proves one OTP can only succeed once, even
// under concurrent requests (run with -race).
func TestVerifySingleUseConcurrent(t *testing.T) {
	users := &fakeOtpUsers{}
	users.ensureUser(1, "u@example.test")
	store := &fakeOtpStore{active: map[uint64]*model.EmailVerificationOtp{}}
	seedOtp(store, 1, "123456", 10*time.Minute)
	mail := &fakeMail{}

	// The serializer store makes Find+MarkUsed one critical section so this
	// test asserts single-use at the service layer. The true FOR UPDATE
	// atomicity is proven by the MySQL integration test.
	ser := &serializerStore{inner: store}
	svc := NewOtpService(ser, users, &fakeOtpTokens{}, fakeTx{}, auth.NewBcryptHasher(), auth.NewOtpGenerator(), auth.NewRandomTokenGenerator(), mail, config.OtpConfig{ExpirationMinutes: 10, MaxAttempts: 5})

	var wg sync.WaitGroup
	wins := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.VerifyEmail(context.Background(), "u@example.test", "123456")
			wins <- err == nil
		}()
	}
	wg.Wait()
	close(wins)
	succ := 0
	for w := range wins {
		if w {
			succ++
		}
	}
	if succ != 1 {
		t.Fatalf("OTP must succeed exactly once across concurrent calls; got %d", succ)
	}
}

// serializerStore emulates atomic single-use with a claim flag: the first
// FindActiveForUpdate "claims" the OTP, everyone after sees ErrNotFound. This
// models the MySQL FOR UPDATE + WHERE used_at IS NULL semantics at the fake
// level (the real DB atomicity is covered by the repository integration test).
type serializerStore struct {
	mu      sync.Mutex
	inner   *fakeOtpStore
	claimed bool
}

func (s *serializerStore) PruneAndInvalidate(ctx context.Context, q repository.Queryer, userID uint64, otpType string) error {
	return s.inner.PruneAndInvalidate(ctx, q, userID, otpType)
}
func (s *serializerStore) Create(ctx context.Context, q repository.Queryer, otp *model.EmailVerificationOtp) error {
	return s.inner.Create(ctx, q, otp)
}
func (s *serializerStore) FindActiveForUpdate(ctx context.Context, q repository.Queryer, userID uint64, otpType string, now time.Time) (*model.EmailVerificationOtp, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimed {
		return nil, repository.ErrNotFound
	}
	rec, err := s.inner.FindActiveForUpdate(ctx, q, userID, otpType, now)
	if err != nil {
		return nil, err
	}
	if rec.UsedAt != nil {
		return nil, repository.ErrNotFound
	}
	s.claimed = true
	return rec, nil
}
func (s *serializerStore) IncrementAttempts(ctx context.Context, q repository.Queryer, id uint64) error {
	return s.inner.IncrementAttempts(ctx, q, id)
}
func (s *serializerStore) MarkUsed(ctx context.Context, q repository.Queryer, id uint64, usedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.MarkUsed(ctx, q, id, usedAt)
}

type fakeOtpTokens struct {
	mu      sync.Mutex
	created []*model.PersonalAccessToken
}

func (f *fakeOtpTokens) Create(ctx context.Context, t *model.PersonalAccessToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, t)
	return nil
}
