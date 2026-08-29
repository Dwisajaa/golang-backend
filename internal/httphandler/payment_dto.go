package httphandler

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type rejectPaymentRequest struct {
	AdminNote string `json:"admin_note"`
}

// paymentData mirrors PaymentResource.
type paymentData struct {
	ID             uint64      `json:"id"`
	InvoiceID      uint64      `json:"invoice_id"`
	PaymentCode    string      `json:"payment_code"`
	PaymentMethod  string      `json:"payment_method"`
	Amount         string      `json:"amount"`
	Status         string      `json:"status"`
	ProofAvailable bool        `json:"proof_available"`
	CustomerNote   *string     `json:"customer_note"`
	AdminNote      *string     `json:"admin_note"`
	PaidAt         timeMicro   `json:"paid_at"`
	VerifiedAt     timeMicro   `json:"verified_at"`
	VerifiedBy     *uint64     `json:"verified_by"`
	Invoice        *payInvoice `json:"invoice"`
}

type payInvoice struct {
	ID            uint64 `json:"id"`
	InvoiceNumber string `json:"invoice_number"`
	TotalAmount   string `json:"total_amount"`
	Status        string `json:"status"`
}

func toPaymentData(p *model.Payment, actorRole string) paymentData {
	d := paymentData{
		ID: p.ID, InvoiceID: p.InvoiceID, PaymentCode: p.PaymentCode,
		PaymentMethod: p.PaymentMethod, Amount: centsToString(p.AmountCents),
		Status:         p.Status,
		ProofAvailable: p.ProofImage != nil && *p.ProofImage != "",
		CustomerNote:   p.CustomerNote, AdminNote: p.AdminNote,
		PaidAt: timeMicro{t: p.PaidAt}, VerifiedAt: timeMicro{t: p.VerifiedAt},
		VerifiedBy: p.VerifiedBy,
	}
	// verified_by is only exposed to admins (Laravel whenLoaded conditional).
	if actorRole != model.RoleAdmin && actorRole != model.RoleSuperAdmin {
		d.VerifiedBy = nil
	}
	if p.Invoice != nil {
		d.Invoice = &payInvoice{
			ID: p.Invoice.ID, InvoiceNumber: p.Invoice.InvoiceNumber,
			TotalAmount: centsToString(p.Invoice.TotalAmountCents), Status: p.Invoice.Status,
		}
	}
	return d
}

func buildPaymentPage(c *gin.Context, list *service.PaymentList, actorRole string) gin.H {
	path := c.Request.URL.Path
	page := list.Page
	last := 0
	if list.PerPage > 0 {
		last = (list.Total + list.PerPage - 1) / list.PerPage
	}
	from, to := 0, 0
	if list.Total > 0 {
		from = (page-1)*list.PerPage + 1
		to = from + len(list.Items) - 1
	}
	link := func(p int) any {
		if p >= 1 && p <= last {
			return path + "?page=" + strconv.Itoa(p)
		}
		return nil
	}
	data := make([]paymentData, 0, len(list.Items))
	for _, p := range list.Items {
		data = append(data, toPaymentData(p, actorRole))
	}
	return gin.H{
		"data":  data,
		"links": gin.H{"first": link(1), "last": link(last), "prev": link(page - 1), "next": link(page + 1)},
		"meta":  gin.H{"current_page": page, "from": from, "last_page": last, "path": path, "per_page": list.PerPage, "to": to, "total": list.Total},
	}
}

// parseMoneyStr converts a decimal money string to integer cents.
func parseMoneyStr(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errBadMoney
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return 0, errBadMoney
	}
	cents := new(big.Rat).Mul(r, big.NewRat(100, 1))
	q := new(big.Int).Quo(cents.Num(), cents.Denom())
	return q.Int64(), nil
}

var errBadMoney = errBadPrice // shared sentinel for numeric validation
