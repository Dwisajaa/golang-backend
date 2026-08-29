package httphandler

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type createBookingRequest struct {
	ItemType          string `json:"item_type"`
	ServiceID         uint64 `json:"service_id"`
	PackageID         uint64 `json:"package_id"`
	Quantity          int    `json:"quantity"`
	BookingDate       string `json:"booking_date"`
	BookingTime       string `json:"booking_time"`
	Address           string `json:"address"`
	AddressDetail     string `json:"address_detail"`
	Latitude          string `json:"latitude"`
	Longitude         string `json:"longitude"`
	CustomerNote      string `json:"customer_note"`
	AdditionalJobdesk string `json:"additional_jobdesk"`
}

func validateCreateBooking(req createBookingRequest) map[string][]string {
	errs := map[string][]string{}
	each := func(f string, m ...string) {
		if len(m) > 0 {
			errs[f] = append(errs[f], m...)
		}
	}
	if req.ItemType == "" {
		each("item_type", "The item type field is required.")
	} else if req.ItemType != "service" && req.ItemType != "package" {
		each("item_type", "The selected item type is invalid.")
	}
	if req.ItemType == "service" && req.ServiceID == 0 {
		each("service_id", "The service id field is required when item type is service.")
	}
	if req.ItemType == "package" && req.PackageID == 0 {
		each("package_id", "The package id field is required when item type is package.")
	}
	if req.Quantity < 1 {
		each("quantity", "The quantity field must be at least 1.")
	} else if req.Quantity > 99 {
		each("quantity", fmt.Sprintf(msgMax, "quantity", 99))
	}
	if req.BookingDate == "" {
		each("booking_date", "The booking date field is required.")
	} else {
		if d, err := time.Parse("2006-01-02", req.BookingDate); err != nil {
			each("booking_date", "The booking date field must be a valid date.")
		} else if d.Before(time.Now().UTC().Truncate(24 * time.Hour)) {
			each("booking_date", "The booking date field must be a date after or equal to today.")
		}
	}
	if req.BookingTime == "" {
		each("booking_time", "The booking time field is required.")
	} else {
		valid := false
		for _, ts := range model.BookingTimeSlots {
			if req.BookingTime == ts {
				valid = true
				break
			}
		}
		if !valid {
			each("booking_time", "The selected booking time is invalid.")
		}
	}
	if req.Address == "" {
		each("address", fmt.Sprintf(msgRequired, "address"))
	} else if len(req.Address) > 255 {
		each("address", fmt.Sprintf(msgMax, "address", 255))
	}
	if req.AddressDetail != "" && len(req.AddressDetail) > 255 {
		each("address_detail", fmt.Sprintf(msgMax, "address detail", 255))
	}
	if req.CustomerNote != "" && len(req.CustomerNote) > 2000 {
		each("customer_note", fmt.Sprintf(msgMax, "customer note", 2000))
	}
	if req.AdditionalJobdesk != "" && len(req.AdditionalJobdesk) > 2000 {
		each("additional_jobdesk", fmt.Sprintf(msgMax, "additional jobdesk", 2000))
	}
	return errs
}

func toCreateBookingInput(req createBookingRequest) service.CreateBookingInput {
	return service.CreateBookingInput{
		ItemType:          req.ItemType,
		ServiceID:         req.ServiceID,
		PackageID:         req.PackageID,
		Quantity:          req.Quantity,
		BookingDate:       req.BookingDate,
		BookingTime:       req.BookingTime,
		Address:           req.Address,
		AddressDetail:     req.AddressDetail,
		Latitude:          req.Latitude,
		Longitude:         req.Longitude,
		CustomerNote:      req.CustomerNote,
		AdditionalJobdesk: req.AdditionalJobdesk,
	}
}

// bookingData mirrors BookingResource.
type bookingData struct {
	ID                uint64            `json:"id"`
	BookingCode       string            `json:"booking_code"`
	BookingDate       string            `json:"booking_date"`
	BookingTime       string            `json:"booking_time"`
	Address           string            `json:"address"`
	AddressDetail     *string           `json:"address_detail"`
	Latitude          *string           `json:"latitude"`
	Longitude         *string           `json:"longitude"`
	CustomerNote      *string           `json:"customer_note"`
	AdditionalJobdesk *string           `json:"additional_jobdesk"`
	Subtotal          string            `json:"subtotal"`
	AdditionalCost    string            `json:"additional_cost"`
	TotalPrice        string            `json:"total_price"`
	Status            string            `json:"status"`
	Items             []bookingItemData `json:"items"`
	Invoice           *invoiceData      `json:"invoice"`
}

type bookingItemData struct {
	ID        uint64  `json:"id"`
	ServiceID *uint64 `json:"service_id"`
	PackageID *uint64 `json:"package_id"`
	ItemType  string  `json:"item_type"`
	ItemName  string  `json:"item_name"`
	Quantity  int     `json:"quantity"`
	UnitPrice string  `json:"unit_price"`
	Subtotal  string  `json:"subtotal"`
}

type invoiceData struct {
	ID             uint64    `json:"id"`
	BookingID      uint64    `json:"booking_id"`
	InvoiceNumber  string    `json:"invoice_number"`
	IssuedAt       timeMicro `json:"issued_at"`
	DueAt          timeMicro `json:"due_at"`
	Subtotal       string    `json:"subtotal"`
	AdditionalCost string    `json:"additional_cost"`
	TotalAmount    string    `json:"total_amount"`
	Status         string    `json:"status"`
	Notes          *string   `json:"notes"`
}

func toBookingData(b *model.Booking) bookingData {
	items := make([]bookingItemData, 0, len(b.Items))
	for _, it := range b.Items {
		items = append(items, bookingItemData{
			ID: it.ID, ServiceID: it.ServiceID, PackageID: it.PackageID,
			ItemType: it.ItemType, ItemName: it.ItemName, Quantity: it.Quantity,
			UnitPrice: centsToString(it.UnitPriceCents), Subtotal: centsToString(it.SubtotalCents),
		})
	}
	d := bookingData{
		ID: b.ID, BookingCode: b.BookingCode,
		BookingDate: b.BookingDate, BookingTime: b.BookingTime,
		Address: b.Address, AddressDetail: b.AddressDetail,
		Latitude: b.Latitude, Longitude: b.Longitude,
		CustomerNote: b.CustomerNote, AdditionalJobdesk: b.AdditionalJobdesk,
		Subtotal:       centsToString(b.SubtotalCents),
		AdditionalCost: centsToString(b.AdditionalCostCents),
		TotalPrice:     centsToString(b.TotalPriceCents),
		Status:         b.Status, Items: items,
	}
	if b.Invoice != nil {
		d.Invoice = &invoiceData{
			ID: b.Invoice.ID, BookingID: b.Invoice.BookingID,
			InvoiceNumber:  b.Invoice.InvoiceNumber,
			IssuedAt:       timeMicro{t: b.Invoice.IssuedAt},
			DueAt:          timeMicro{t: b.Invoice.DueAt},
			Subtotal:       centsToString(b.Invoice.SubtotalCents),
			AdditionalCost: centsToString(b.Invoice.AdditionalCostCents),
			TotalAmount:    centsToString(b.Invoice.TotalAmountCents),
			Status:         b.Invoice.Status, Notes: b.Invoice.Notes,
		}
	}
	return d
}

func buildBookingPage(c *gin.Context, list *service.BookingList) gin.H {
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
			q := c.Request.URL.Query()
			q.Set("page", strconv.Itoa(p))
			return path + "?" + q.Encode()
		}
		return nil
	}
	data := make([]bookingData, 0, len(list.Items))
	for _, b := range list.Items {
		data = append(data, toBookingData(b))
	}
	return gin.H{
		"data":  data,
		"links": gin.H{"first": link(1), "last": link(last), "prev": link(page - 1), "next": link(page + 1)},
		"meta":  gin.H{"current_page": page, "from": from, "last_page": last, "path": path, "per_page": list.PerPage, "to": to, "total": list.Total},
	}
}
