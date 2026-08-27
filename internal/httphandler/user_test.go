package httphandler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
)

type stubUserService struct {
	user *model.User
	err  error
}

func (s stubUserService) GetUserByID(ctx context.Context, id uint64) (*model.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.user, nil
}

func userRouter(svc UserService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewUserHandler(svc)
	r.GET("/api/users/:id", h.Get)
	return r
}

func testUser() *model.User {
	return &model.User{
		ID: 1, Name: "Dev Customer", Email: "customer@example.test", Role: model.RoleCustomer,
	}
}

func TestGetUserRoute(t *testing.T) {
	rec := httptest.NewRecorder()
	userRouter(stubUserService{user: testUser()}).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users/1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	data := body["data"]
	if data["id"].(float64) != 1 || data["email"] != "customer@example.test" || data["role"] != "customer" {
		t.Fatalf("unexpected payload: %+v", data)
	}
	if _, has := data["password"]; has {
		t.Fatal("password leaked in response")
	}
	if _, has := data["remember_token"]; has {
		t.Fatal("remember_token leaked in response")
	}
}

func TestGetUserInvalidIDReturns404(t *testing.T) {
	for _, path := range []string{"/api/users/abc", "/api/users/0"} {
		rec := httptest.NewRecorder()
		userRouter(stubUserService{user: testUser()}).
			ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: expected 404, got %d", path, rec.Code)
		}
	}
}

func TestGetUserNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	userRouter(stubUserService{err: httperr.NotFound("Resource not found.")}).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users/9", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if body["message"] != "Resource not found." {
		t.Fatalf("unexpected message: %s", body["message"])
	}
}

func TestGetUserInternalErrorIs500AndGeneric(t *testing.T) {
	rec := httptest.NewRecorder()
	userRouter(stubUserService{err: httperr.Internal(ErrChars("driver: connection reset"))}).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users/1", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != `{"message":"Server error."}` {
		t.Fatalf("unexpected body: %s", body)
	}
}

// ErrChars is a plain error type so the test asserts the driver detail is never
// transmitted to the client.
type ErrChars string

func (e ErrChars) Error() string { return string(e) }
