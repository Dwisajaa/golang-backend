package service

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type fakeTechProfiles struct {
	byUser         map[uint64]*model.TechnicianProfile
	insertCount    int
	codeCollisions int // first N inserts return ErrDuplicateTechnicianCode
	userCollision  bool
	err            error
	lastUpdated    *model.TechnicianProfile
}

func (f *fakeTechProfiles) FindByUserID(ctx context.Context, q repository.Queryer, userID uint64) (*model.TechnicianProfile, error) {
	if f.err != nil {
		return nil, f.err
	}
	if p, ok := f.byUser[userID]; ok {
		return p, nil
	}
	return nil, repository.ErrNotFound
}

func (f *fakeTechProfiles) InsertProfile(ctx context.Context, q repository.Queryer, p *model.TechnicianProfile) error {
	if f.err != nil {
		return f.err
	}
	f.insertCount++
	if f.codeCollisions > 0 {
		f.codeCollisions--
		return repository.ErrDuplicateTechnicianCode
	}
	if f.userCollision {
		f.userCollision = false
		ex := &model.TechnicianProfile{ID: 1, UserID: p.UserID, TechnicianCode: "TECH-0001"}
		f.byUser[p.UserID] = ex
		return repository.ErrDuplicate
	}
	if _, exists := f.byUser[p.UserID]; exists {
		return repository.ErrDuplicate
	}
	p.ID = uint64(len(f.byUser) + 1)
	f.byUser[p.UserID] = p
	return nil
}

func (f *fakeTechProfiles) UpdateProfile(ctx context.Context, q repository.Queryer, userID uint64, p *model.TechnicianProfile) error {
	if f.err != nil {
		return f.err
	}
	f.lastUpdated = p
	return nil
}

var techCodeRE = regexp.MustCompile(`^TECH-\d{4}$`)

func TestTechProfileFound(t *testing.T) {
	phone := "0811"
	fake := &fakeTechProfiles{byUser: map[uint64]*model.TechnicianProfile{
		7: {ID: 1, UserID: 7, TechnicianCode: "TECH-0001", Phone: &phone, IsActive: true},
	}}
	svc := NewTechnicianProfileService(fake, fakeTx{})

	p, err := svc.GetByUserID(context.Background(), 7)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if p.TechnicianCode != "TECH-0001" {
		t.Fatalf("unexpected profile: %+v", p)
	}
}

func TestTechProfileMissingIs404(t *testing.T) {
	fake := &fakeTechProfiles{byUser: map[uint64]*model.TechnicianProfile{}}
	svc := NewTechnicianProfileService(fake, fakeTx{})

	_, err := svc.GetByUserID(context.Background(), 7)
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindNotFound || he.Message != "Technician profile not found." {
		t.Fatalf("expected 404 with Laravel message, got %v", err)
	}
}

func TestTechProfileCreateGeneratesCode(t *testing.T) {
	fake := &fakeTechProfiles{byUser: map[uint64]*model.TechnicianProfile{}}
	svc := NewTechnicianProfileService(fake, fakeTx{})

	p, err := svc.UpdateProfile(context.Background(), 7, TechnicianProfileInput{
		Phone: "0811", Specialization: "AC", Address: "Jl", Bio: "Bio",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !techCodeRE.MatchString(p.TechnicianCode) {
		t.Fatalf("expected TECH-XXXX code, got %q", p.TechnicianCode)
	}
	if p.Phone == nil || p.Specialization == nil || p.Address == nil || p.Bio == nil {
		t.Fatalf("non-empty fields must be stored, got %+v", p)
	}
}

func TestTechProfileEmptyFieldsStoredNull(t *testing.T) {
	fake := &fakeTechProfiles{byUser: map[uint64]*model.TechnicianProfile{}}
	svc := NewTechnicianProfileService(fake, fakeTx{})

	p, err := svc.UpdateProfile(context.Background(), 7, TechnicianProfileInput{})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if p.Phone != nil || p.Specialization != nil || p.Address != nil || p.Bio != nil {
		t.Fatalf("empty optional fields must stay NULL, got %+v", p)
	}
}

func TestTechProfileUpdateExisting(t *testing.T) {
	fake := &fakeTechProfiles{byUser: map[uint64]*model.TechnicianProfile{
		7: {ID: 1, UserID: 7, TechnicianCode: "TECH-0042", IsActive: true},
	}}
	svc := NewTechnicianProfileService(fake, fakeTx{})

	p, err := svc.UpdateProfile(context.Background(), 7, TechnicianProfileInput{Phone: "0812"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if fake.lastUpdated == nil {
		t.Fatal("expected update path")
	}
	if p.TechnicianCode != "TECH-0042" {
		t.Fatalf("existing code must be preserved, got %q", p.TechnicianCode)
	}
	if p.Phone == nil || *p.Phone != "0812" {
		t.Fatalf("update not reflected: %+v", p)
	}
}

func TestTechProfileCodeCollisionRetries(t *testing.T) {
	fake := &fakeTechProfiles{
		byUser:         map[uint64]*model.TechnicianProfile{},
		codeCollisions: 2, // first two generated codes collide
	}
	svc := NewTechnicianProfileService(fake, fakeTx{})

	p, err := svc.UpdateProfile(context.Background(), 7, TechnicianProfileInput{Phone: "0813"})
	if err != nil {
		t.Fatalf("update with collisions: %v", err)
	}
	if !techCodeRE.MatchString(p.TechnicianCode) {
		t.Fatalf("bad code after retries: %q", p.TechnicianCode)
	}
	if fake.insertCount != 3 {
		t.Fatalf("expected 3 inserts (2 collisions + success), got %d", fake.insertCount)
	}
}

func TestTechProfileConcurrentCreateFallsBackToUpdate(t *testing.T) {
	fake := &fakeTechProfiles{byUser: map[uint64]*model.TechnicianProfile{}, userCollision: true}
	svc := NewTechnicianProfileService(fake, fakeTx{})

	p, err := svc.UpdateProfile(context.Background(), 7, TechnicianProfileInput{Phone: "0814"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	// the concurrent creator's row is used, its profile fields updated
	if p.TechnicianCode != "TECH-0001" {
		t.Fatalf("expected concurrent row code, got %q", p.TechnicianCode)
	}
	if fake.lastUpdated == nil {
		t.Fatal("expected update path after duplicate user_id")
	}
	if p.Phone == nil || *p.Phone != "0814" {
		t.Fatalf("fields not applied after fallback: %+v", p)
	}
}

func TestTechProfileRepositoryError(t *testing.T) {
	fake := &fakeTechProfiles{byUser: map[uint64]*model.TechnicianProfile{}, err: errors.New("db down")}
	svc := NewTechnicianProfileService(fake, fakeTx{})

	if _, err := svc.GetByUserID(context.Background(), 7); httperr.As(err) == nil || httperr.As(err).Kind != httperr.KindInternal {
		t.Fatalf("expected internal error, got %v", err)
	}
	if _, err := svc.UpdateProfile(context.Background(), 7, TechnicianProfileInput{Phone: "x"}); httperr.As(err) == nil || httperr.As(err).Kind != httperr.KindInternal {
		t.Fatalf("expected internal error, got %v", err)
	}
}
