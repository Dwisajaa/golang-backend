package httphandler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type InvoiceService interface {
	ListByCustomer(ctx context.Context, customerID uint64, page, perPage int) (*service.InvoiceList, error)
	Show(ctx context.Context, customerID, invoiceID uint64) (*model.Invoice, error)
}

type InvoiceHandler struct {
	service InvoiceService
}

func NewInvoiceHandler(svc InvoiceService) *InvoiceHandler { return &InvoiceHandler{service: svc} }

// List serves GET /api/invoices (customer).
func (h *InvoiceHandler) List(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	page, perPage := parsePagination(c)
	list, err := h.service.ListByCustomer(c.Request.Context(), u.ID, page, perPage)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, buildInvoicePage(c, list))
}

// Show serves GET /api/invoices/:id (customer, policy view).
func (h *InvoiceHandler) Show(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	inv, err := h.service.Show(c.Request.Context(), u.ID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toInvoiceData(inv)})
}

func toInvoiceData(inv *model.Invoice) invoiceData {
	return invoiceData{
		ID:             inv.ID,
		BookingID:      inv.BookingID,
		InvoiceNumber:  inv.InvoiceNumber,
		IssuedAt:       timeMicro{t: inv.IssuedAt},
		DueAt:          timeMicro{t: inv.DueAt},
		Subtotal:       centsToString(inv.SubtotalCents),
		AdditionalCost: centsToString(inv.AdditionalCostCents),
		TotalAmount:    centsToString(inv.TotalAmountCents),
		Status:         inv.Status,
		Notes:          inv.Notes,
	}
}

func buildInvoicePage(c *gin.Context, list *service.InvoiceList) gin.H {
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
	data := make([]invoiceData, 0, len(list.Items))
	for _, inv := range list.Items {
		data = append(data, toInvoiceData(inv))
	}
	return gin.H{
		"data":  data,
		"links": gin.H{"first": link(1), "last": link(last), "prev": link(page - 1), "next": link(page + 1)},
		"meta":  gin.H{"current_page": page, "from": from, "last_page": last, "path": path, "per_page": list.PerPage, "to": to, "total": list.Total},
	}
}
