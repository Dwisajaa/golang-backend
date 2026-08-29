package httphandler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
	"github.com/Dwisajaa/golang-backend/internal/storage"
)

type PaymentService interface {
	UploadProof(ctx context.Context, customerID, invoiceID uint64, in service.UploadProofInput) (*model.Payment, error)
	ShowProofByInvoice(ctx context.Context, customerID, invoiceID uint64) (*model.Payment, error)
	ShowProofByID(ctx context.Context, userID uint64, role string, paymentID uint64) (*model.Payment, error)
	AdminList(ctx context.Context, status string, page, perPage int) (*service.PaymentList, error)
	Verify(ctx context.Context, adminID, paymentID uint64) (*model.Payment, error)
	Reject(ctx context.Context, adminID, paymentID uint64, adminNote string) (*model.Payment, error)
}

type PaymentHandler struct {
	service PaymentService
	storage storage.Storage
}

func NewPaymentHandler(svc PaymentService, st storage.Storage) *PaymentHandler {
	return &PaymentHandler{service: svc, storage: st}
}

const maxProofBytes = 2048 * 1024 // Laravel max:2048 (KB)

func (h *PaymentHandler) StoreProof(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	invoiceID, ok := parseID(c)
	if !ok {
		return
	}

	var errs = map[string][]string{}
	each := func(f string, msgs ...string) {
		if len(msgs) > 0 {
			errs[f] = append(errs[f], msgs...)
		}
	}

	paymentMethod := c.PostForm("payment_method")
	amountStr := c.PostForm("amount")
	customerNote := c.PostForm("customer_note")

	amountCents, moneyErr := parseMoneyStr(amountStr)
	switch {
	case paymentMethod == "":
		each("payment_method", "The payment method field is required.")
	case paymentMethod != model.PaymentMethodBankTransfer:
		each("payment_method", "The selected payment method is invalid.")
	}
	switch {
	case amountStr == "":
		each("amount", "The amount field is required.")
	case moneyErr != nil:
		each("amount", "The amount field must be a number.")
	case amountCents <= 0:
		each("amount", "The amount field must be at least 0.01.")
	}
	if customerNote != "" && len(customerNote) > 1000 {
		each("customer_note", "The customer note field must not be greater than 1000 characters.")
	}

	fileHeader, err := c.FormFile("proof_image")
	if err != nil {
		each("proof_image", "The proof image field is required.")
	}
	if len(errs) > 0 {
		respondError(c, httperr.Validation(errs))
		return
	}
	if fileHeader != nil {
		if fileHeader.Size > maxProofBytes {
			each("proof_image", "The proof image must not be greater than 2048 kilobytes.")
		} else {
			f, openErr := fileHeader.Open()
			if openErr != nil {
				respondError(c, httperr.Internal(openErr))
				return
			}
			defer f.Close()
			// content-type validation mirrors Laravel mimetypes. Read first bytes.
			head := make([]byte, 512)
			n, _ := io.ReadFull(f, head)
			head = head[:n]
			if !validImageMagic(head) {
				each("proof_image", "The proof image must be a file of type: jpg, jpeg, png.")
			}
			_, _ = f.Seek(0, io.SeekStart)
			if len(errs) > 0 {
				respondError(c, httperr.Validation(errs))
				return
			}
			payment, err := h.service.UploadProof(c.Request.Context(), u.ID, invoiceID, service.UploadProofInput{
				PaymentMethod: paymentMethod,
				AmountCents:   amountCents,
				CustomerNote:  customerNote,
				ProofFile:     f,
				ProofFilename: fileHeader.Filename,
			})
			if err != nil {
				respondError(c, err)
				return
			}
			c.JSON(http.StatusCreated, gin.H{
				"message": "Payment proof uploaded successfully.",
				"data":    toPaymentData(payment, model.RoleCustomer),
			})
			return
		}
	}
	respondError(c, httperr.Validation(errs))
}

// ShowProofByInvoice serves GET /api/invoices/:id/payment-proof (customer).
func (h *PaymentHandler) ShowProofByInvoice(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	invoiceID, ok := parseID(c)
	if !ok {
		return
	}
	payment, err := h.service.ShowProofByInvoice(c.Request.Context(), u.ID, invoiceID)
	if err != nil {
		respondError(c, err)
		return
	}
	h.serveProof(c, payment)
}

// ShowProofByID serves GET /api/admin/payments/:id/proof.
func (h *PaymentHandler) ShowProofByID(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	paymentID, ok := parseID(c)
	if !ok {
		return
	}
	payment, err := h.service.ShowProofByID(c.Request.Context(), u.ID, u.Role, paymentID)
	if err != nil {
		respondError(c, err)
		return
	}
	h.serveProof(c, payment)
}

func (h *PaymentHandler) serveProof(c *gin.Context, payment *model.Payment) {
	if payment.ProofImage == nil {
		respondError(c, httperr.NotFound("Payment proof not found."))
		return
	}
	path, err := h.storage.Path(*payment.ProofImage)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			respondError(c, httperr.NotFound("Payment proof not found."))
			return
		}
		respondError(c, httperr.Internal(err))
		return
	}
	c.Header("Content-Disposition", "inline; filename=\"payment-proof-"+strconv.FormatUint(payment.ID, 10)+"\"")
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(path)
}

func (h *PaymentHandler) AdminList(c *gin.Context) {
	page, perPage := parsePagination(c)
	status := strings.TrimSpace(c.Query("status"))
	list, err := h.service.AdminList(c.Request.Context(), status, page, perPage)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, buildPaymentPage(c, list, model.RoleSuperAdmin))
}

func (h *PaymentHandler) Verify(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	paymentID, ok := parseID(c)
	if !ok {
		return
	}
	p, err := h.service.Verify(c.Request.Context(), u.ID, paymentID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Payment verified successfully.",
		"data":    toPaymentData(p, model.RoleSuperAdmin),
	})
}

func (h *PaymentHandler) Reject(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	paymentID, ok := parseID(c)
	if !ok {
		return
	}
	var req rejectPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return
	}
	adminNote := strings.TrimSpace(req.AdminNote)
	if adminNote == "" {
		respondError(c, httperr.Validation(map[string][]string{"admin_note": {"The admin note field is required."}}))
		return
	}
	if len(adminNote) > 1000 {
		respondError(c, httperr.Validation(map[string][]string{"admin_note": {"The admin note field must not be greater than 1000 characters."}}))
		return
	}
	p, err := h.service.Reject(c.Request.Context(), u.ID, paymentID, adminNote)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Payment rejected successfully.",
		"data":    toPaymentData(p, model.RoleSuperAdmin),
	})
}

// validImageMagic sniffs image magic bytes (JPEG/PNG).
func validImageMagic(b []byte) bool {
	if len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF {
		return true // jpg
	}
	if len(b) >= 8 && b[0] == 0x89 && string(b[1:4]) == "PNG" {
		return true
	}
	return false
}
