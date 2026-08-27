package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type stubPinger struct {
	err error
}

func (s stubPinger) PingContext(ctx context.Context) error { return s.err }

func readyRouter(status error) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewReadyHandler(stubPinger{err: status})
	r.GET("/ready", h.Ready)
	return r
}

func TestReadyWhenDBUp(t *testing.T) {
	rec := httptest.NewRecorder()
	readyRouter(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if body["status"] != "ready" {
		t.Fatalf("expected status=ready, got %#v", body)
	}
}

func TestReadyWhenDBDown(t *testing.T) {
	rec := httptest.NewRecorder()
	readyRouter(errors.New("connection refused")).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if body["status"] != "unavailable" {
		t.Fatalf("expected status=unavailable, got %#v", body)
	}
}
