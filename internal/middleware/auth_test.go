package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/auth"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type stubTokenStore struct {
	byHash map[string]*model.PersonalAccessToken
	err    error
}

func (s *stubTokenStore) FindByTokenHash(ctx context.Context, hash string) (*model.PersonalAccessToken, error) {
	if s.err != nil {
		return nil, s.err
	}
	if t, ok := s.byHash[hash]; ok {
		return t, nil
	}
	return nil, repository.ErrNotFound
}

type stubUserFinder struct {
	byID map[uint64]*model.User
	err  error
}

func (s *stubUserFinder) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	if u, ok := s.byID[id]; ok {
		return u, nil
	}
	return nil, repository.ErrNotFound
}

func newAuthMW() (gin.HandlerFunc, *stubTokenStore, *stubUserFinder) {
	ts := &stubTokenStore{byHash: map[string]*model.PersonalAccessToken{}}
	uf := &stubUserFinder{byID: map[uint64]*model.User{}}
	return Auth(ts, uf, auth.NewRandomTokenGenerator()), ts, uf
}

func buildRouter(mw gin.HandlerFunc, handler func(*gin.Context)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mw)
	r.GET("/probe", func(c *gin.Context) { handler(c) })
	return r
}

func signedToken(ts *stubTokenStore, raw string, expiresAt *time.Time) *model.PersonalAccessToken {
	gen := auth.NewRandomTokenGenerator()
	t := &model.PersonalAccessToken{
		ID: 1, TokenableType: model.TokenableType, TokenableID: 7,
		Name: "mobile-app", Token: gen.Hash(raw), ExpiresAt: expiresAt,
	}
	ts.byHash[t.Token] = t
	return t
}

func TestMissingAuthorizationHeader(t *testing.T) {
	mw, _, _ := newAuthMW()
	called := false
	r := buildRouter(mw, func(c *gin.Context) { called = true; c.Status(200) })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probe", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if called {
		t.Fatal("handler must not be called")
	}
	if body := rec.Body.String(); body != `{"message":"Unauthenticated."}` {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestNotBearerScheme(t *testing.T) {
	for _, h := range []string{"Basic abc", "Bearer", "Bearer   ", "bearer xyz"} {
		mw, _, _ := newAuthMW()
		called := false
		r := buildRouter(mw, func(c *gin.Context) { called = true })
		req := httptest.NewRequest(http.MethodGet, "/probe", nil)
		req.Header.Set("Authorization", h)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%q: expected 401, got %d", h, rec.Code)
		}
		if called {
			t.Errorf("%q: handler called", h)
		}
	}
}

func TestValidTokenPassesAndStoresUser(t *testing.T) {
	mw, ts, uf := newAuthMW()
	signedToken(ts, "valid-raw-token-123", ptrTime(time.Now().UTC().Add(time.Hour)))
	uf.byID[7] = &model.User{ID: 7, Name: "U", Email: "u@example.test", Role: model.RoleCustomer}

	var gotID uint64
	r := buildRouter(mw, func(c *gin.Context) {
		u, ok := CurrentUser(c)
		if !ok {
			t.Error("CurrentUser missing")
		}
		gotID = u.ID
		c.Status(200)
	})
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Authorization", "Bearer valid-raw-token-123")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotID != 7 {
		t.Fatalf("expected user id 7 in context, got %d", gotID)
	}
}

func TestUnknownToken401(t *testing.T) {
	mw, _, _ := newAuthMW()
	called := false
	r := buildRouter(mw, func(c *gin.Context) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Authorization", "Bearer no-such-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if called {
		t.Fatal("handler called for unknown token")
	}
}

func TestExpiredToken401(t *testing.T) {
	mw, ts, uf := newAuthMW()
	signedToken(ts, "expired-raw", ptrTime(time.Now().UTC().Add(-time.Hour)))
	uf.byID[7] = &model.User{ID: 7}

	called := false
	r := buildRouter(mw, func(c *gin.Context) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Authorization", "Bearer expired-raw")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if called {
		t.Fatal("handler called for expired token")
	}
}

func TestTokenOwnerMissing401(t *testing.T) {
	mw, ts, _ := newAuthMW()
	signedToken(ts, "valid-token-no-user", ptrTime(time.Now().UTC().Add(time.Hour))) // owner id 7 not seeded

	called := false
	r := buildRouter(mw, func(c *gin.Context) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Authorization", "Bearer valid-token-no-user")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if called {
		t.Fatal("handler called when token owner missing")
	}
}

func TestAuthenticationFailureDoesNotLeakTokenMaterial(t *testing.T) {
	mw, _, _ := newAuthMW()
	r := buildRouter(mw, func(c *gin.Context) { c.Status(200) })
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Authorization", "Bearer some-magic-token-abc")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != `{"message":"Unauthenticated."}` {
		t.Fatalf("unexpected body: %s", body)
	}
	if strings.Contains(rec.Body.String(), "some-magic-token-abc") {
		t.Fatal("raw token leaked in response")
	}
}

// TestConcurrentRequestsIsolateUsers proves request-scoped isolation. Under
// -race it also proves there is no shared mutable authentication state.
func TestConcurrentRequestsIsolateUsers(t *testing.T) {
	gen := auth.NewRandomTokenGenerator()
	exp := ptrTime(time.Now().UTC().Add(time.Hour))

	ts := &stubTokenStore{byHash: map[string]*model.PersonalAccessToken{}}
	uf := &stubUserFinder{byID: map[uint64]*model.User{}}

	const n = 40
	for i := uint64(1); i <= n; i++ {
		raw := rawToken(i)
		ts.byHash[gen.Hash(raw)] = &model.PersonalAccessToken{
			ID: i, TokenableType: model.TokenableType, TokenableID: i,
			Name: "mobile-app", Token: gen.Hash(raw), ExpiresAt: exp,
		}
		uf.byID[i] = &model.User{ID: i, Name: "U", Email: "u@example.test", Role: model.RoleCustomer}
	}

	r := buildRouter(Auth(ts, uf, gen), func(c *gin.Context) {
		u, ok := CurrentUser(c)
		if !ok {
			t.Error("no user in context")
			return
		}
		time.Sleep(time.Millisecond) // encourage interleaving
		if u.ID == 0 {
			t.Error("bad user id")
		}
	})

	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := uint64(1); i <= n; i++ {
		wg.Add(1)
		go func(id uint64) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/probe", nil)
			req.Header.Set("Authorization", "Bearer "+rawToken(id))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				errs <- fmt.Errorf("user %d got status %d", id, rec.Code)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

func rawToken(i uint64) string { return "raw-token-" + strconv.FormatUint(i, 10) }

func ptrTime(t time.Time) *time.Time { return &t }
