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

type stubCatService struct {
	list *service.CategoryList
	err  error
	cat  *model.ServiceCategory
}

func (s *stubCatService) ListCategories(ctx context.Context, page int) (*service.CategoryList, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.list != nil {
		return s.list, nil
	}
	return &service.CategoryList{Items: nil, Total: 0, Page: 1, PerPage: model.DefaultCategoryPerPage}, nil
}
func (s *stubCatService) Create(ctx context.Context, in service.CategoryInput) (*model.ServiceCategory, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.cat, nil
}
func (s *stubCatService) Update(ctx context.Context, id uint64, in service.CategoryInput) (*model.ServiceCategory, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.cat, nil
}
func (s *stubCatService) Delete(ctx context.Context, id uint64) error { return s.err }

func catRouter(svc ServiceCategoryService, user *model.User, roleAuth bool) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	raw := "cat-token"
	gen := auth.NewRandomTokenGenerator()
	exp := time.Now().UTC().Add(time.Hour)
	u := user
	if u == nil {
		u = &model.User{ID: 1, Role: model.RoleAdmin}
	}
	h := NewServiceCategoryHandler(svc)

	api := r.Group("/api")
	api.GET("/categories", h.List)
	if roleAuth {
		admin := api.Group("")
		admin.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: u.ID, exp: &exp}, &authUserStub{user: u}, gen))
		admin.Use(middleware.RequireRole(model.RoleAdmin, model.RoleSuperAdmin))
		admin.POST("/admin/categories", h.Store)
		admin.PUT("/admin/categories/:id", h.Update)
		admin.DELETE("/admin/categories/:id", h.Destroy)
	}
	return r, raw
}

func TestCatListPublic(t *testing.T) {
	desc := ""
	svc := &stubCatService{list: &service.CategoryList{
		Items: []*model.ServiceCategory{{
			ID: 1, Name: "A", Slug: "a", Description: &desc, IsActive: true,
			Services: []*model.ServiceLite{{ID: 5, Name: "S", Slug: "s", PriceCents: 15000, Unit: "per_service", IsActive: true}},
		}},
		Total: 1, Page: 1, PerPage: 15,
	}}
	r, _ := catRouter(svc, nil, false)
	rec := doAuth(t, r, http.MethodGet, "/api/categories", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	meta := body["meta"].(map[string]any)
	if meta["total"].(float64) != 1 || meta["per_page"].(float64) != 15 {
		t.Fatalf("meta wrong: %v", meta)
	}
	data := body["data"].([]any)
	first := data[0].(map[string]any)
	if first["name"] != "A" {
		t.Fatalf("category wrong: %v", first)
	}
	svcs := first["services"].([]any)
	s0 := svcs[0].(map[string]any)
	// price serialized as DECIMAL string (Laravel decimal:2 cast)
	if s0["price"] != "150.00" {
		t.Fatalf("price must be a 2-dp string, got %v", s0["price"])
	}
	if s0["category"] != nil {
		t.Fatalf("embedded service category must be null, got %v", s0["category"])
	}
}

func TestCatAdminCreate(t *testing.T) {
	svc := &stubCatService{cat: &model.ServiceCategory{ID: 1, Name: "A", Slug: "a", IsActive: true}}
	r, raw := catRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/categories", `{"name":"A"}`, raw)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Category created successfully.") {
		t.Fatalf("message missing: %s", rec.Body.String())
	}
}

func TestCatAdminUnauthenticated(t *testing.T) {
	svc := &stubCatService{}
	r, _ := catRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/categories", `{"name":"A"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestCatAdminWrongRole(t *testing.T) {
	svc := &stubCatService{}
	r, raw := catRouter(svc, &model.User{ID: 9, Role: model.RoleCustomer}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/categories", `{"name":"A"}`, raw)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestCatAdminValidation(t *testing.T) {
	svc := &stubCatService{}
	r, raw := catRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/categories", `{"name":""}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestCatAdminNonBooleanIsActive(t *testing.T) {
	svc := &stubCatService{cat: &model.ServiceCategory{ID: 1, Name: "A", Slug: "a"}}
	r, raw := catRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/categories", `{"name":"A","is_active":"yes"}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "must be true or false") {
		t.Fatalf("expected boolean rule message: %s", rec.Body.String())
	}
}

func TestCatAdminUpdateAndDelete(t *testing.T) {
	svc := &stubCatService{cat: &model.ServiceCategory{ID: 1, Name: "A", Slug: "a"}}
	r, raw := catRouter(svc, &model.User{ID: 9, Role: model.RoleSuperAdmin}, true)

	rec := doAuth(t, r, http.MethodPut, "/api/admin/categories/1", `{"name":"B"}`, raw)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Category updated successfully.") {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}

	rec = doAuth(t, r, http.MethodDelete, "/api/admin/categories/1", "", raw)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Category deleted successfully.") {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCatAdminConflict(t *testing.T) {
	svc := &stubCatService{err: httperr.Conflict("Category cannot be deleted while it has services. Deactivate it instead.")}
	r, raw := catRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodDelete, "/api/admin/categories/1", "", raw)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestCatListInternalError(t *testing.T) {
	svc := &stubCatService{err: httperr.Internal(ErrBooms{})}
	r, _ := catRouter(svc, nil, false)
	rec := doAuth(t, r, http.MethodGet, "/api/categories", "", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Server error.") {
		t.Fatalf("detail leaked: %s", rec.Body.String())
	}
}
