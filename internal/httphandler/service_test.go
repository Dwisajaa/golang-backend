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

type stubSvcService struct {
	list *service.ServiceList
	svc  *model.Service
	err  error
}

func (s *stubSvcService) List(ctx context.Context, categoryID *uint64, search string, page, perPage int) (*service.ServiceList, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.list != nil {
		return s.list, nil
	}
	return &service.ServiceList{Items: nil, Total: 0, Page: 1, PerPage: model.DefaultServicePerPage}, nil
}
func (s *stubSvcService) Get(ctx context.Context, id uint64) (*model.Service, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.svc, nil
}
func (s *stubSvcService) Create(ctx context.Context, in service.ServiceInput) (*model.Service, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.svc, nil
}
func (s *stubSvcService) Update(ctx context.Context, id uint64, in service.ServiceInput) (*model.Service, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.svc, nil
}
func (s *stubSvcService) Delete(ctx context.Context, id uint64) error { return s.err }

func serviceRouter(svc ServiceService, user *model.User, admin bool) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	raw := "svc-token"
	gen := auth.NewRandomTokenGenerator()
	exp := time.Now().UTC().Add(time.Hour)
	u := user
	if u == nil {
		u = &model.User{ID: 1, Role: model.RoleAdmin}
	}
	h := NewServiceHandler(svc)

	api := r.Group("/api")
	api.GET("/services", h.List)
	api.GET("/services/:id", h.Get)
	if admin {
		adm := api.Group("")
		adm.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: u.ID, exp: &exp}, &authUserStub{user: u}, gen))
		adm.Use(middleware.RequireRole(model.RoleAdmin, model.RoleSuperAdmin))
		adm.POST("/admin/services", h.Store)
		adm.PUT("/admin/services/:id", h.Update)
		adm.DELETE("/admin/services/:id", h.Destroy)
	}
	return r, raw
}

func sampleService() *model.Service {
	desc := ""
	est := int64(60)
	return &model.Service{
		ID: 3, ServiceCategoryID: 1, Name: "AC", Slug: "ac",
		Description: &desc, PriceCents: 15000, Unit: "per_service",
		EstimatedDuration: &est, IsActive: true,
		Category: &model.ServiceCategory{ID: 1, Name: "Cat", Slug: "cat"},
	}
}

func TestSvcListPublic(t *testing.T) {
	svc := &stubSvcService{list: &service.ServiceList{
		Items: []*model.Service{sampleService()}, Total: 1, Page: 1, PerPage: 15,
	}}
	r, _ := serviceRouter(svc, nil, false)
	rec := doAuth(t, r, http.MethodGet, "/api/services", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	meta := body["meta"].(map[string]any)
	if meta["per_page"].(float64) != 15 {
		t.Fatalf("per_page wrong: %v", meta)
	}
	first := body["data"].([]any)[0].(map[string]any)
	if first["price"] != "150.00" {
		t.Fatalf("price must be string 2dp, got %v", first["price"])
	}
	cat, ok := first["category"].(map[string]any)
	if !ok || cat["name"] != "Cat" {
		t.Fatalf("category nested missing: %v", first["category"])
	}
	if len(cat["services"].([]any)) != 0 {
		t.Fatal("nested category services must be []")
	}
}

func TestSvcDetail404WhenInactive(t *testing.T) {
	svc := &stubSvcService{err: httperr.NotFound("Resource not found.")}
	r, _ := serviceRouter(svc, nil, false)
	rec := doAuth(t, r, http.MethodGet, "/api/services/9", "", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestSvcAdminCreate(t *testing.T) {
	svc := &stubSvcService{svc: sampleService()}
	r, raw := serviceRouter(svc, &model.User{ID: 9, Role: model.RoleSuperAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/services",
		`{"service_category_id":1,"name":"AC","price":15000,"unit":"per_service"}`, raw)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "Service created successfully.") {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSvcAdminUnauthorized403(t *testing.T) {
	svc := &stubSvcService{svc: sampleService()}
	r, raw := serviceRouter(svc, &model.User{ID: 9, Role: model.RoleTechnician}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/services",
		`{"service_category_id":1,"name":"AC","price":15000,"unit":"per_service"}`, raw)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestSvcAdminValidationShape(t *testing.T) {
	svc := &stubSvcService{}
	r, raw := serviceRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/services",
		`{"service_category_id":0,"name":"","price":-1,"unit":"bogus"}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	errs := body["errors"].(map[string]any)
	for _, f := range []string{"service_category_id", "name", "price", "unit"} {
		if len(errs[f].([]any)) == 0 {
			t.Fatalf("expected error for %s: %v", f, errs)
		}
	}
}

func TestSvcAdminNonNumericPrice(t *testing.T) {
	svc := &stubSvcService{}
	r, raw := serviceRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/services",
		`{"service_category_id":1,"name":"A","price":"abc","unit":"per_service"}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "must be a number") {
		t.Fatalf("expected numeric message: %s", rec.Body.String())
	}
}

func TestSvcAdminCategoryInvalid(t *testing.T) {
	svc := &stubSvcService{err: httperr.Validation(map[string][]string{
		"service_category_id": {"The selected service category id is invalid."},
	})}
	r, raw := serviceRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/services",
		`{"service_category_id":99,"name":"A","price":1,"unit":"per_service"}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestSvcAdminConflict(t *testing.T) {
	svc := &stubSvcService{err: httperr.Conflict("Service cannot be deleted while it is used by a package. Deactivate it instead.")}
	r, raw := serviceRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodDelete, "/api/admin/services/1", "", raw)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestSvcInternalErrorGeneric(t *testing.T) {
	svc := &stubSvcService{err: httperr.Internal(ErrBooms{})}
	r, _ := serviceRouter(svc, nil, false)
	rec := doAuth(t, r, http.MethodGet, "/api/services", "", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Server error.") {
		t.Fatalf("detail leaked: %s", rec.Body.String())
	}
}

func TestSvcMalformed(t *testing.T) {
	svc := &stubSvcService{}
	r, raw := serviceRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/services", `{"name":`, raw)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
