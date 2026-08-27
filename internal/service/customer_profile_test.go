package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type fakeCustomerProfiles struct {
	byUser map[uint64]*model.CustomerProfile
	err    error
}

func (f *fakeCustomerProfiles) FindByUserID(ctx context.Context, q repository.Queryer, userID uint64) (*model.CustomerProfile, error) {
	if f.err != nil {
		return nil, f.err
	}
	if p, ok := f.byUser[userID]; ok {
		return p, nil
	}
	return nil, repository.ErrNotFound
}

func (f *fakeCustomerProfiles) Upsert(ctx context.Context, q repository.Queryer, p *model.CustomerProfile) error {
	if f.err != nil {
		return f.err
	}
	f.byUser[p.UserID] = p
	return nil
}

func TestCustomerProfileMissingReturnsNil(t *testing.T) {
	fake := &fakeCustomerProfiles{byUser: map[uint64]*model.CustomerProfile{}}
	svc := NewCustomerProfileService(fake, fakeTx{})

	p, err := svc.GetByUserID(context.Background(), 7)
	if err != nil {
		t.Fatalf("missing profile is not an error (Laravel data:null): %v", err)
	}
	if p != nil {
		t.Fatalf("expected nil, got %+v", p)
	}
}

func TestCustomerProfileFound(t *testing.T) {
	fake := &fakeCustomerProfiles{byUser: map[uint64]*model.CustomerProfile{
		7: {ID: 1, UserID: 7, FullName: "A", Phone: "0812", Address: "Jl. X", City: "Jakarta"},
	}}
	svc := NewCustomerProfileService(fake, fakeTx{})

	p, err := svc.GetByUserID(context.Background(), 7)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if p.ID != 1 || p.FullName != "A" {
		t.Fatalf("unexpected profile: %+v", p)
	}
}

func TestCustomerProfileUpsertCreatesFirstTime(t *testing.T) {
	fake := &fakeCustomerProfiles{byUser: map[uint64]*model.CustomerProfile{}}
	svc := NewCustomerProfileService(fake, fakeTx{})

	p, err := svc.Upsert(context.Background(), 7, UpdateInput{
		FullName: "Full", Phone: "0812", Address: "Addr", City: "City",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if p.UserID != 7 {
		t.Fatalf("profile must belong to the authenticated user: %+v", p)
	}
	if p.PostalCode != nil {
		t.Fatal("empty postal_code must be stored as NULL")
	}
	if !p.IsComplete() {
		t.Fatal("complete profile must be is_complete")
	}
}

func TestCustomerProfileUpsertUpdatesExisting(t *testing.T) {
	postal := "12345"
	fake := &fakeCustomerProfiles{byUser: map[uint64]*model.CustomerProfile{
		7: {ID: 1, UserID: 7, FullName: "Old", Phone: "0812", Address: "A", City: "C", PostalCode: &postal},
	}}
	svc := NewCustomerProfileService(fake, fakeTx{})

	p, err := svc.Upsert(context.Background(), 7, UpdateInput{
		FullName: "New", Phone: "0813", Address: "B", City: "D", PostalCode: "54321",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got := fake.byUser[7]
	if got.FullName != "New" || got.Phone != "0813" || got.PostalCode == nil || *got.PostalCode != "54321" {
		t.Fatalf("update not applied: %+v", got)
	}
	if p.PostalCode == nil || *p.PostalCode != "54321" {
		t.Fatalf("returned profile postal mismatch: %+v", p)
	}
}

func TestCustomerProfileRepositoryError(t *testing.T) {
	fake := &fakeCustomerProfiles{byUser: map[uint64]*model.CustomerProfile{}, err: errors.New("db down")}
	svc := NewCustomerProfileService(fake, fakeTx{})

	if _, err := svc.GetByUserID(context.Background(), 7); err == nil {
		t.Fatal("expected 500-class error on repo failure")
	}
	if _, err := svc.Upsert(context.Background(), 7, UpdateInput{FullName: "A", Phone: "1", Address: "A", City: "C"}); err == nil {
		t.Fatal("expected error on upsert repo failure")
	}
}
