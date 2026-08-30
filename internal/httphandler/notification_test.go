package httphandler

import (
	"context"
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

type stubNotifSvc struct {
	list *service.NotificationList
	n    *model.Notification
	err  error
}

func (s *stubNotifSvc) List(ctx context.Context, uid uint64, p, pp int) (*service.NotificationList, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.list != nil {
		return s.list, nil
	}
	return &service.NotificationList{Total: 0, Page: 1, PerPage: 15}, nil
}
func (s *stubNotifSvc) Read(ctx context.Context, uid uint64, id string) (*model.Notification, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.n, nil
}
func (s *stubNotifSvc) ReadAll(ctx context.Context, uid uint64) error { return s.err }

func notifRouter(svc NotificationService, user *model.User) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	raw := "notif-token"
	gen := auth.NewRandomTokenGenerator()
	exp := time.Now().UTC().Add(time.Hour)
	u := user
	if u == nil {
		u = &model.User{ID: 7, Role: model.RoleCustomer}
	}
	h := NewNotificationHandler(svc)
	api := r.Group("/api")
	pr := api.Group("")
	pr.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: u.ID, exp: &exp}, &authUserStub{user: u}, gen))
	pr.GET("/notifications", h.List)
	pr.POST("/notifications/read-all", h.ReadAll)
	pr.POST("/notifications/:id/read", h.Read)
	return r, raw
}

func TestNotificationListShape(t *testing.T) {
	svc := &stubNotifSvc{list: &service.NotificationList{
		Items: []*model.Notification{{ID: "uuid-1", Type: model.NotificationType, Data: model.SystemNotification{
			Event: "payment_verified", Title: "Payment verified", Message: "Your payment...",
			BookingID: &[]uint64{9}[0], PaymentID: &[]uint64{5}[0],
		}}}, Total: 1, Page: 1, PerPage: 15,
	}}
	r, raw := notifRouter(svc, nil)
	rec := doAuth(t, r, http.MethodGet, "/api/notifications", "", raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"type":"payment_verified"`) || !strings.Contains(rec.Body.String(), `"message":"Your payment..."`) {
		t.Fatalf("shape wrong: %s", rec.Body.String())
	}
}

func TestNotificationRead(t *testing.T) {
	svc := &stubNotifSvc{n: &model.Notification{ID: "n1", Data: model.SystemNotification{Event: "e"}}}
	r, raw := notifRouter(svc, nil)
	rec := doAuth(t, r, http.MethodPost, "/api/notifications/n1/read", "", raw)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Notification marked as read.") {
		t.Fatalf("read: %d %s", rec.Code, rec.Body.String())
	}
}

func TestNotificationReadAll(t *testing.T) {
	svc := &stubNotifSvc{}
	r, raw := notifRouter(svc, nil)
	rec := doAuth(t, r, http.MethodPost, "/api/notifications/read-all", "", raw)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "All notifications marked as read.") {
		t.Fatalf("read-all: %d %s", rec.Code, rec.Body.String())
	}
}

func TestNotificationUnauthorized(t *testing.T) {
	svc := &stubNotifSvc{}
	r, _ := notifRouter(svc, nil)
	rec := doAuth(t, r, http.MethodGet, "/api/notifications", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestNotificationNotFound(t *testing.T) {
	svc := &stubNotifSvc{err: httperr.NotFound("Resource not found.")}
	r, raw := notifRouter(svc, nil)
	rec := doAuth(t, r, http.MethodPost, "/api/notifications/x/read", "", raw)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
