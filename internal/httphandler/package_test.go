package httphandler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/auth"
	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type stubPkgService struct {
	list *service.PackageList
	pkg  *model.Package
	err  error
}

func (s *stubPkgService) List(ctx context.Context, search string, page, perPage int) (*service.PackageList, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.list != nil {
		return s.list, nil
	}
	return &service.PackageList{Items: nil, Total: 0, Page: 1, PerPage: model.DefaultPackagePerPage}, nil
}
func (s *stubPkgService) Get(ctx context.Context, id uint64) (*model.Package, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.pkg, nil
}
func (s *stubPkgService) Create(ctx context.Context, in service.PackageInput) (*model.Package, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.pkg, nil
}
func (s *stubPkgService) Update(ctx context.Context, id uint64, in service.PackageInput) (*model.Package, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.pkg, nil
}
func (s *stubPkgService) Delete(ctx context.Context, id uint64) error { return s.err }

func samplePkg() *model.Package {
	return &model.Package{
		ID: 1, Name: "Premium AC", Slug: "premium-ac", PriceCents: 30000,
		IsActive: true, IsPopular: true,
		Items: []*model.PackageItem{{
			ID: 10, PackageID: 1, ServiceID: 5, Quantity: 2,
			Service: &model.Service{ID: 5, Name: "AC Svc", Slug: "ac-svc", PriceCents: 15000, Unit: "per_service", IsActive: true},
		}},
	}
}

func pkgRouter(svc PackageService, user *model.User, admin bool) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	raw := "pkg-token"
	gen := auth.NewRandomTokenGenerator()
	exp := time.Now().UTC().Add(time.Hour)
	u := user
	if u == nil {
		u = &model.User{ID: 1, Role: model.RoleAdmin}
	}
	h := NewPackageHandler(svc)
	api := r.Group("/api")
	api.GET("/packages", h.List)
	api.GET("/packages/:id", h.Get)
	if admin {
		adm := api.Group("")
		adm.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: u.ID, exp: &exp}, &authUserStub{user: u}, gen))
		adm.Use(middleware.RequireRole(model.RoleAdmin, model.RoleSuperAdmin))
		adm.POST("/admin/packages", h.Store)
		adm.PUT("/admin/packages/:id", h.Update)
		adm.DELETE("/admin/packages/:id", h.Destroy)
	}
	return r, raw
}

func TestPkgListPublic(t *testing.T) {
	svc := &stubPkgService{list: &service.PackageList{
		Items: []*model.Package{samplePkg()}, Total: 1, Page: 1, PerPage: 15,
	}}
	r, _ := pkgRouter(svc, nil, false)
	rec := doAuth(t, r, http.MethodGet, "/api/packages", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	first := body["data"].([]any)[0].(map[string]any)
	if first["price"] != "300.00" {
		t.Fatalf("price: %v", first["price"])
	}
	if first["is_popular"] != true {
		t.Fatalf("is_popular: %v", first["is_popular"])
	}
	items := first["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items: %v", items)
	}
	item0 := items[0].(map[string]any)
	svcNested := item0["service"].(map[string]any)
	if svcNested["price"] != "150.00" {
		t.Fatalf("nested service price: %v", svcNested["price"])
	}
}

func TestPkgDetail404(t *testing.T) {
	svc := &stubPkgService{err: httperr.NotFound("Resource not found.")}
	r, _ := pkgRouter(svc, nil, false)
	rec := doAuth(t, r, http.MethodGet, "/api/packages/99", "", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestPkgAdminCreate(t *testing.T) {
	svc := &stubPkgService{pkg: samplePkg()}
	r, raw := pkgRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/packages",
		`{"name":"Premium AC","price":300,"items":[{"service_id":5,"quantity":2}]}`, raw)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "Package created successfully.") {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPkgAdminValidation(t *testing.T) {
	svc := &stubPkgService{}
	r, raw := pkgRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/packages", `{"name":"","price":-1,"items":[]}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	errs := body["errors"].(map[string]any)
	for _, f := range []string{"name", "price", "items"} {
		if _, ok := errs[f]; !ok {
			t.Fatalf("expected error for %s: %v", f, errs)
		}
	}
}

func TestPkgAdminDelete(t *testing.T) {
	svc := &stubPkgService{}
	r, raw := pkgRouter(svc, &model.User{ID: 9, Role: model.RoleSuperAdmin}, true)
	rec := doAuth(t, r, http.MethodDelete, "/api/admin/packages/1", "", raw)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Package deleted successfully.") {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPkgAdmin403(t *testing.T) {
	svc := &stubPkgService{pkg: samplePkg()}
	r, raw := pkgRouter(svc, &model.User{ID: 9, Role: model.RoleCustomer}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/packages",
		`{"name":"X","price":1,"items":[{"service_id":1,"quantity":1}]}`, raw)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestPkgInternalError(t *testing.T) {
	svc := &stubPkgService{err: httperr.Internal(ErrBooms{})}
	r, _ := pkgRouter(svc, nil, false)
	rec := doAuth(t, r, http.MethodGet, "/api/packages", "", "")
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "Server error.") {
		t.Fatalf("500: %d %s", rec.Code, rec.Body.String())
	}
}
