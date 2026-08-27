package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

func userCtx(u *model.User) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ctxKeyUser, u)
		c.Next()
	}
}

func TestRequireRoleAllows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(userCtx(&model.User{ID: 1, Role: model.RoleCustomer}))
	r.Use(RequireRole(model.RoleCustomer))
	called := false
	r.GET("/x", func(c *gin.Context) { called = true; c.Status(200) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("expected pass-through, got %d called=%v", rec.Code, called)
	}
}

func TestRequireRoleBlocksWrongRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(userCtx(&model.User{ID: 1, Role: model.RoleTechnician}))
	r.Use(RequireRole(model.RoleCustomer))
	called := false
	r.GET("/x", func(c *gin.Context) { called = true })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if called {
		t.Fatal("handler must not run for wrong role")
	}
	if rec.Body.String() != `{"message":"Forbidden."}` {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestRequireRoleAllowsMultiple(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(userCtx(&model.User{ID: 1, Role: model.RoleSuperAdmin}))
	r.Use(RequireRole(model.RoleAdmin, model.RoleSuperAdmin))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("role gate must pass to 404 route handling, got %d", rec.Code)
	}
}
