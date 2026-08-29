package httphandler

import (
	"github.com/Dwisajaa/golang-backend/internal/model"
)

// assignmentData mirrors BookingAssignmentResource.
type assignmentData struct {
	ID              uint64             `json:"id"`
	BookingID       uint64             `json:"booking_id"`
	TechnicianID    uint64             `json:"technician_id"`
	AssignedBy      *uint64            `json:"assigned_by"`
	Status          string             `json:"status"`
	AssignedAt      timeMicro          `json:"assigned_at"`
	AcceptedAt      timeMicro          `json:"accepted_at"`
	RejectedAt      timeMicro          `json:"rejected_at"`
	StartedAt       timeMicro          `json:"started_at"`
	CompletedAt     timeMicro          `json:"completed_at"`
	RejectionReason *string            `json:"rejection_reason"`
	TechnicianNote  *string            `json:"technician_note"`
	Booking         *assignmentBooking `json:"booking"`
}

type assignmentBooking struct {
	ID            uint64            `json:"id"`
	BookingCode   string            `json:"booking_code"`
	BookingDate   string            `json:"booking_date"`
	BookingTime   string            `json:"booking_time"`
	Address       string            `json:"address"`
	AddressDetail *string           `json:"address_detail"`
	CustomerNote  *string           `json:"customer_note"`
	Status        string            `json:"status"`
	Items         []bookingItemData `json:"items"`
	Customer      *assignCustomer   `json:"customer"`
}

type assignCustomer struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func toAssignmentData(a *model.BookingAssignment) assignmentData {
	d := assignmentData{
		ID: a.ID, BookingID: a.BookingID, TechnicianID: a.TechnicianID,
		AssignedBy: a.AssignedBy, Status: a.Status,
		AssignedAt: timeMicro{t: a.AssignedAt}, AcceptedAt: timeMicro{t: a.AcceptedAt},
		RejectedAt: timeMicro{t: a.RejectedAt}, StartedAt: timeMicro{t: a.StartedAt},
		CompletedAt:     timeMicro{t: a.CompletedAt},
		RejectionReason: a.RejectionReason, TechnicianNote: a.TechnicianNote,
	}
	if a.Booking != nil {
		items := make([]bookingItemData, 0, len(a.Booking.Items))
		for _, it := range a.Booking.Items {
			items = append(items, bookingItemData{
				ID: it.ID, ServiceID: it.ServiceID, PackageID: it.PackageID,
				ItemType: it.ItemType, ItemName: it.ItemName, Quantity: it.Quantity,
				UnitPrice: centsToString(it.UnitPriceCents), Subtotal: centsToString(it.SubtotalCents),
			})
		}
		b := &assignmentBooking{
			ID: a.Booking.ID, BookingCode: a.Booking.BookingCode,
			BookingDate: a.Booking.BookingDate, BookingTime: a.Booking.BookingTime,
			Address: a.Booking.Address, AddressDetail: a.Booking.AddressDetail,
			CustomerNote: a.Booking.CustomerNote, Status: a.Booking.Status,
			Items: items,
		}
		if a.Booking.Customer != nil {
			b.Customer = &assignCustomer{
				ID: a.Booking.Customer.ID, Name: a.Booking.Customer.Name, Email: a.Booking.Customer.Email,
			}
		}
		d.Booking = b
	}
	return d
}
