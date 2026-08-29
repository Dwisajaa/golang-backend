package httphandler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/auth"
	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type stubInvoiceSvc struct {
	list *service.InvoiceList
	inv  *model.Invoice
	err  error
}

func (s *stubInvoiceSvc) ListByCustomer(ctx context.Context, cid uint64, p, pp int) (*service.InvoiceList, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.list != nil {
		return s.list, nil
	}
	return &service.InvoiceList{Total: 0, Page: 1, PerPage: 15}, nil
}
func (s *stubInvoiceSvc) Show(ctx context.Context, cid, iid uint64) (*model.Invoice, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.inv, nil
}

func invoiceRouter(svc InvoiceService, user *model.User) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	raw := "inv-token"
	gen := auth.NewRandomTokenGenerator()
	exp := time.Now().UTC().Add(time.Hour)
	u := user
	if u == nil {
		u = &model.User{ID: 7, Role: model.RoleCustomer}
	}
	h := NewInvoiceHandler(svc)

	api := r.Group("/api")
	cust := api.Group("")
	cust.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: u.ID, exp: &exp}, &authUserStub{user: u}, gen))
	cust.Use(middleware.RequireRole(model.RoleCustomer))
	cust.GET("/invoices", h.List)
	cust.GET("/invoices/:id", h.Show)
	return r, raw
}

func sampleInvoice() *model.Invoice {
	return &model.Invoice{
		ID: 1, BookingID: 10, InvoiceNumber: "INV-BJA-20261201-0001-0001",
		SubtotalCents: 30000, TotalAmountCents: 30000, Status: model.InvoiceStatusUnpaid,
	}
}

func TestInvoiceListShape(t *testing.T) {
	svc := &stubInvoiceSvc{list: &service.InvoiceList{
		Items: []*model.Invoice{sampleInvoice()}, Total: 1, Page: 1, PerPage: 15,
	}}
	r, raw := invoiceRouter(svc, nil)
	rec := doAuth(t, r, http.MethodGet, "/api/invoices", "", raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	data := body["data"].([]any)
	first := data[0].(map[string]any)
	if first["invoice_number"] != "INV-BJA-20261201-0001-0001" || first["total_amount"] != "300.00" || first["status"] != "unpaid" {
		t.Fatalf("shape wrong: %+v", first)
	}
}

func TestInvoiceShowOk(t *testing.T) {
	svc := &stubInvoiceSvc{inv: sampleInvoice()}
	r, raw := invoiceRouter(svc, nil)
	rec := doAuth(t, r, http.MethodGet, "/api/invoices/1", "", raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestInvoiceShow403(t *testing.T) {
	svc := &stubInvoiceSvc{err: httperr.Forbidden("Forbidden.")}
	r, raw := invoiceRouter(svc, nil)
	rec := doAuth(t, r, http.MethodGet, "/api/invoices/1", "", raw)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestInvoiceShow404(t *testing.T) {
	svc := &stubInvoiceSvc{err: httperr.NotFound("Resource not found.")}
	r, raw := invoiceRouter(svc, nil)
	rec := doAuth(t, r, http.MethodGet, "/api/invoices/99", "", raw)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestInvoiceUnauthenticated(t *testing.T) {
	svc := &stubInvoiceSvc{}
	r, _ := invoiceRouter(svc, nil)
	rec := doAuth(t, r, http.MethodGet, "/api/invoices", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestInvoiceWrongRole(t *testing.T) {
	svc := &stubInvoiceSvc{}
	r, raw := invoiceRouter(svc, &model.User{ID: 9, Role: model.RoleTechnician})
	rec := doAuth(t, r, http.MethodGet, "/api/invoices", "", raw)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestInvoiceInternalError(t *testing.T) {
	svc := &stubInvoiceSvc{err: httperr.Internal(ErrBooms{})}
	r, raw := invoiceRouter(svc, nil)
	rec := doAuth(t, r, http.MethodGet, "/api/invoices", "", raw)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
