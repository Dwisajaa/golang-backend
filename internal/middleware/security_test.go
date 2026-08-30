package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimitBlocksAfterLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", RateLimit("test-blocks", 3, time.Minute), func(c *gin.Context) { c.Status(200) })

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d should pass: %d", i+1, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if rec.Body.String() != `{"message":"Too many requests. Please try again later."}` {
		t.Fatalf("body: %s", rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestRateLimitResetsAfterWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", RateLimit("test-reset", 1, time.Minute), func(c *gin.Context) { c.Status(200) })

	hit := func() int {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
		return rec.Code
	}
	first := hit()
	second := hit()
	if first != 200 || second != 429 {
		t.Fatalf("window semantics wrong: first=%d second=%d", first, second)
	}
	// second window (fresh key) must pass
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatal("new client must not share the counter")
	}
}

func TestRateLimitConcurrentSafe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", RateLimit("test-race", 100, time.Minute), func(c *gin.Context) { c.Status(200) })

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
		}()
	}
	wg.Wait()
}

func TestLimitBodyCapsOversized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(LimitBody(10))
	r.POST("/x", func(c *gin.Context) {
		// reading past the cap must error (the cap is enforced before the
		// handler/parser gets any chance to allocate unbounded memory)
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(200)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader([]byte(strings.Repeat("a", 64))))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected read error past the cap, got %d", rec.Code)
	}
}

func TestLimitBodyAllowsSmall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(LimitBody(64))
	r.POST("/x", func(c *gin.Context) { c.Status(200) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader([]byte("hello"))))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRecoveryReturnsGenericJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	r := gin.New()
	r.Use(JSONRecovery(logger))
	r.GET("/boom", func(c *gin.Context) { panic("kaboom") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if rec.Body.String() != `{"message":"Server error."}` {
		t.Fatalf("body: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "kaboom") || strings.Contains(rec.Body.String(), "panic") {
		t.Fatal("stack/panic leaked to client")
	}
}

func TestCORSAllowedAndPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(OriginAllowlist([]string{"https://app.example.test"})))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	// allowed simple request
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://app.example.test")
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Access-Control-Allow-Origin") != "https://app.example.test" {
		t.Fatalf("allowed origin headers wrong: %d %q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}

	// disallowed origin â†’ no CORS headers
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://evil.example.test")
	r.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("disallowed origin must not receive CORS headers")
	}
}

func TestCORSPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(OriginAllowlist([]string{"https://app.example.test"})))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.Header.Set("Origin", "https://app.example.test")
	req.Header.Set("Access-Control-Request-Method", "GET")
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 preflight, got %d", rec.Code)
	}
}
