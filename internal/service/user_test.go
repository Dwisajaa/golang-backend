package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type fakeUserRepo struct {
	user *model.User
	err  error
}

func (f fakeUserRepo) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.user != nil && f.user.ID == id {
		return f.user, nil
	}
	return nil, repository.ErrNotFound
}

func TestGetUserByIDFound(t *testing.T) {
	want := &model.User{ID: 7, Name: "A", Email: "a@example.test", Role: model.RoleCustomer}
	svc := NewUserService(fakeUserRepo{user: want})

	got, err := svc.GetUserByID(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != want.ID || got.Name != "A" {
		t.Fatalf("wrong user: %+v", got)
	}
}

func TestGetUserByIDNotFound(t *testing.T) {
	svc := NewUserService(fakeUserRepo{err: repository.ErrNotFound})
	_, err := svc.GetUserByID(context.Background(), 42)
	if err == nil {
		t.Fatal("expected error")
	}
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindNotFound {
		t.Fatalf("expected typed NotFound, got %#v", err)
	}
}

func TestGetUserByIDRepositoryError(t *testing.T) {
	svc := NewUserService(fakeUserRepo{err: errors.New("boom: connect refused")})
	_, err := svc.GetUserByID(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error")
	}
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindInternal {
		t.Fatalf("expected typed Internal, got %#v", err)
	}
	if he.Message == "boom: connect refused" {
		t.Fatal("internal message must not leak driver details to the client")
	}
	if he.Err == nil {
		t.Fatal("underlying error must be preserved for server-side logging")
	}
}
