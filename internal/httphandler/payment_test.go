package httphandler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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

type stubPaymentSvc struct {
	p    *model.Payment
	list *service.PaymentList
	err  error
	note string
}

func (s *stubPaymentSvc) UploadProof(ctx context.Context, cid, iid uint64, in service.UploadProofInput) (*model.Payment, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.p, nil
}
func (s *stubPaymentSvc) ShowProofByInvoice(ctx context.Context, cid, iid uint64) (*model.Payment, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.p, nil
}
func (s *stubPaymentSvc) ShowProofByID(ctx context.Context, uid uint64, role string, pid uint64) (*model.Payment, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.p, nil
}
func (s *stubPaymentSvc) AdminList(ctx context.Context, status string, p, pp int) (*service.PaymentList, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.list != nil {
		return s.list, nil
	}
	return &service.PaymentList{Total: 0, Page: 1, PerPage: 15}, nil
}
func (s *stubPaymentSvc) Verify(ctx context.Context, aid, pid uint64) (*model.Payment, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.p, nil
}
func (s *stubPaymentSvc) Reject(ctx context.Context, aid, pid uint64, note string) (*model.Payment, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.p, nil
}

func samplePayment() *model.Payment {
	proof := "payment-proof-abc.png"
	return &model.Payment{
		ID: 1, InvoiceID: 1, PaymentCode: "PAY-BJA-X-0001",
		PaymentMethod: model.PaymentMethodBankTransfer, AmountCents: 30000,
		Status: model.PaymentStatusWaitingVerification, ProofImage: &proof,
		Invoice: &model.Invoice{ID: 1, InvoiceNumber: "INV-X-1", TotalAmountCents: 30000, Status: model.InvoiceStatusPendingPayment},
	}
}

func paymentRouter(svc PaymentService, user *model.User, isAdmin bool) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	raw := "pay-token"
	gen := auth.NewRandomTokenGenerator()
	exp := time.Now().UTC().Add(time.Hour)
	u := user
	if u == nil {
		u = &model.User{ID: 7, Role: model.RoleCustomer}
	}
	h := NewPaymentHandler(svc, &memStubStorage{})
	api := r.Group("/api")

	authed := api.Group("")
	authed.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: u.ID, exp: &exp}, &authUserStub{user: u}, gen))
	authed.Use(middleware.RequireRole(model.RoleCustomer))
	authed.POST("/invoices/:id/payment-proof", h.StoreProof)
	authed.GET("/invoices/:id/payment-proof", h.ShowProofByInvoice)

	if isAdmin {
		adm := api.Group("")
		adm.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: u.ID, exp: &exp}, &authUserStub{user: u}, gen))
		adm.Use(middleware.RequireRole(model.RoleAdmin, model.RoleSuperAdmin))
		adm.GET("/admin/payments", h.AdminList)
		adm.GET("/admin/payments/:id/proof", h.ShowProofByID)
		adm.POST("/admin/payments/:id/verify", h.Verify)
		adm.POST("/admin/payments/:id/reject", h.Reject)
	}
	return r, raw
}

// memStubStorage satisfies the storage.Storage interface expected by handler.
type memStubStorage struct{}

func (m *memStubStorage) Save(key string, r io.Reader) error     { return nil }
func (m *memStubStorage) Exists(key string) (bool, error)        { return true, nil }
func (m *memStubStorage) Open(key string) (io.ReadCloser, error) { return nil, nil }
func (m *memStubStorage) Path(key string) (string, error)        { return "proofs/" + key, nil }
func (m *memStubStorage) Delete(key string) error                { return nil }

func multipartBody(t *testing.T, fileBytes []byte, filename string) (*bytes.Buffer, string) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	_ = w.WriteField("payment_method", "bank_transfer")
	_ = w.WriteField("amount", "300.00")
	_ = w.WriteField("customer_note", "proof")
	fh, _ := w.CreateFormFile("proof_image", filename)
	if fileBytes != nil {
		_, _ = fh.Write(fileBytes)
	}
	_ = w.Close()
	return &b, w.FormDataContentType()
}

func doMultipart(t *testing.T, r *gin.Engine, path, token string, body *bytes.Buffer, ct string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestPaymentUpload201(t *testing.T) {
	svc := &stubPaymentSvc{p: samplePayment()}
	r, raw := paymentRouter(svc, nil, false)
	body, ct := multipartBody(t, []byte{0xFF, 0xD8, 0xFF, 0xE0, 1, 2, 3}, "proof.jpg")
	rec := doMultipart(t, r, "/api/invoices/1/payment-proof", raw, body, ct)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Payment proof uploaded successfully.") {
		t.Fatalf("msg: %s", rec.Body.String())
	}
	// no sensitive fields
	if strings.Contains(rec.Body.String(), "payment-proof-abc.png") {
		t.Fatalf("raw storage path leaked: %s", rec.Body.String())
	}
}

func TestPaymentUploadNotJpegMagic(t *testing.T) {
	svc := &stubPaymentSvc{p: samplePayment()}
	r, raw := paymentRouter(svc, nil, false)
	body, ct := multipartBody(t, []byte("plaintext-not-an-image-jpj"), "proof.jpg")
	rec := doMultipart(t, r, "/api/invoices/1/payment-proof", raw, body, ct)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestPaymentUploadValidation(t *testing.T) {
	svc := &stubPaymentSvc{p: samplePayment()}
	r, raw := paymentRouter(svc, nil, false)
	body, ct := multipartBody(t, nil, "proof.jpg")
	rec := doMultipart(t, r, "/api/invoices/1/payment-proof", raw, body, ct)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestPaymentUnauthenticated(t *testing.T) {
	svc := &stubPaymentSvc{p: samplePayment()}
	r, _ := paymentRouter(svc, nil, false)
	body, ct := multipartBody(t, []byte{0xFF, 0xD8, 0xFF, 0xE0}, "proof.jpg")
	rec := doMultipart(t, r, "/api/invoices/1/payment-proof", "", body, ct)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestPaymentAdminVerify(t *testing.T) {
	svc := &stubPaymentSvc{p: samplePayment()}
	r, raw := paymentRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/payments/1/verify", "", raw)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Payment verified successfully.") {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPaymentAdminRejectValidation(t *testing.T) {
	svc := &stubPaymentSvc{}
	r, raw := paymentRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/payments/1/reject", `{"admin_note":""}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestPaymentAdminList(t *testing.T) {
	svc := &stubPaymentSvc{list: &service.PaymentList{Total: 0, Page: 1, PerPage: 15}}
	r, raw := paymentRouter(svc, &model.User{ID: 9, Role: model.RoleSuperAdmin}, true)
	rec := doAuth(t, r, http.MethodGet, "/api/admin/payments", "", raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestPaymentWrongRole403(t *testing.T) {
	svc := &stubPaymentSvc{p: samplePayment()}
	r, raw := paymentRouter(svc, &model.User{ID: 8, Role: model.RoleTechnician}, false)
	body, ct := multipartBody(t, []byte{0xFF, 0xD8, 0xFF, 0xE0}, "proof.jpg")
	rec := doMultipart(t, r, "/api/invoices/1/payment-proof", raw, body, ct)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestPaymentInternalError(t *testing.T) {
	svc := &stubPaymentSvc{err: httperr.Internal(ErrBooms{})}
	r, raw := paymentRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/payments/1/verify", "", raw)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
