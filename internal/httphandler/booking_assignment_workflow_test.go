package httphandler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/auth"
	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
)

func techRouter2(h *AssignmentHandler, user *model.User, raw string) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	gen := auth.NewRandomTokenGenerator()
	exp := time.Now().UTC().Add(time.Hour)
	api := r.Group("/api")
	tech := api.Group("")
	tech.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: user.ID, exp: &exp}, &authUserStub{user: user}, gen))
	tech.Use(middleware.RequireRole(model.RoleTechnician))
	tech.GET("/technician/jobs", h.ListJobs)
	tech.GET("/technician/jobs/:id", h.ShowJob)
	tech.POST("/technician/jobs/:id/accept", h.Accept)
	tech.POST("/technician/jobs/:id/reject", h.Reject)
	tech.POST("/technician/jobs/:id/start", h.Start)
	tech.POST("/technician/jobs/:id/complete", h.Complete)
	return r, raw
}

func TestWorkAcceptRoute(t *testing.T) {
	svc := &stubAssignSvc{a: sampleAssignment()}
	r, raw := techRouter2(NewAssignmentHandler(svc), &model.User{ID: 9, Role: "technician"}, "job-token")
	rec := doAuth(t, r, http.MethodPost, "/api/technician/jobs/5/accept", "", raw)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Assignment accepted successfully.") {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
	}
}

func TestWorkRejectReasonValidation(t *testing.T) {
	svc := &stubAssignSvc{a: sampleAssignment()}
	r, raw := techRouter2(NewAssignmentHandler(svc), &model.User{ID: 9, Role: "technician"}, "job-token")
	rec := doAuth(t, r, http.MethodPost, "/api/technician/jobs/5/reject", `{"rejection_reason":"Bukan alasan"}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestWorkRejectValidReason(t *testing.T) {
	svc := &stubAssignSvc{a: sampleAssignment()}
	r, raw := techRouter2(NewAssignmentHandler(svc), &model.User{ID: 9, Role: "technician"}, "job-token")
	rec := doAuth(t, r, http.MethodPost, "/api/technician/jobs/5/reject", `{"rejection_reason":"Jadwal bentrok","rejection_reason_detail":"besok"}`, raw)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Assignment rejected successfully.") {
		t.Fatalf("reject: %d %s", rec.Code, rec.Body.String())
	}
}

func TestWorkStartCompleteRoutes(t *testing.T) {
	svc := &stubAssignSvc{a: sampleAssignment()}
	r, raw := techRouter2(NewAssignmentHandler(svc), &model.User{ID: 9, Role: "technician"}, "job-token")

	rec := doAuth(t, r, http.MethodPost, "/api/technician/jobs/5/start", "", raw)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Job started successfully.") {
		t.Fatalf("start: %d %s", rec.Code, rec.Body.String())
	}
	rec = doAuth(t, r, http.MethodPost, "/api/technician/jobs/5/complete", `{"technician_note":"OK"}`, raw)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Job completed and awaiting verification.") {
		t.Fatalf("complete: %d %s", rec.Code, rec.Body.String())
	}
}

func TestWorkCompleteNoteRequired(t *testing.T) {
	svc := &stubAssignSvc{a: sampleAssignment()}
	r, raw := techRouter2(NewAssignmentHandler(svc), &model.User{ID: 9, Role: "technician"}, "job-token")
	rec := doAuth(t, r, http.MethodPost, "/api/technician/jobs/5/complete", `{"technician_note":""}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestWorkWrongRole403(t *testing.T) {
	svc := &stubAssignSvc{a: sampleAssignment()}
	r, raw := techRouter2(NewAssignmentHandler(svc), &model.User{ID: 7, Role: "customer"}, "job-token")
	rec := doAuth(t, r, http.MethodPost, "/api/technician/jobs/5/accept", "", raw)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestWorkBusiness422(t *testing.T) {
	svc := &stubAssignSvc{err: httperr.Validation(map[string][]string{"assignment": {"Assignment is not in the required state."}})}
	r, raw := techRouter2(NewAssignmentHandler(svc), &model.User{ID: 9, Role: "technician"}, "job-token")
	rec := doAuth(t, r, http.MethodPost, "/api/technician/jobs/5/accept", "", raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestWorkListJobs(t *testing.T) {
	svc := &stubAssignSvc{}
	r, raw := techRouter2(NewAssignmentHandler(svc), &model.User{ID: 9, Role: "technician"}, "job-token")
	rec := doAuth(t, r, http.MethodGet, "/api/technician/jobs", "", raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
