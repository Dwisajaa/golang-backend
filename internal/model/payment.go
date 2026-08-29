package model

import "time"

// Payment method + status constants mirror Laravel Payment model.
const (
	PaymentMethodBankTransfer = "bank_transfer"

	PaymentStatusUnpaid              = "unpaid"
	PaymentStatusWaitingVerification = "waiting_verification"
	PaymentStatusPending             = "pending"
	PaymentStatusPaid                = "paid"
	PaymentStatusRejected            = "rejected"
	PaymentStatusFailed              = "failed"
	PaymentStatusExpired             = "expired"
	PaymentStatusRefunded            = "refunded"
	PaymentStatusCancelled           = "cancelled"
)

// PaymentPendingVerificationStatuses mirrors
// Payment::pendingVerificationStatuses() — the statuses an admin may act on.
var PaymentPendingVerificationStatuses = []string{
	PaymentStatusWaitingVerification,
	PaymentStatusPending,
}

// IsPaymentPendingVerification reports whether the payment awaits admin action.
func IsPaymentPendingVerification(status string) bool {
	for _, s := range PaymentPendingVerificationStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// PaymentTransitions is the centralized transition table for admin actions.
// Laravel enforces these through ensurePending() + explicit updates rather than
// a transitions() map; this table encodes the same permitted moves.
var PaymentTransitions = map[string][]string{
	PaymentStatusWaitingVerification: {PaymentStatusPaid, PaymentStatusRejected},
	PaymentStatusPending:             {PaymentStatusPaid, PaymentStatusRejected},
}

// CanPaymentTransition reports whether the status change is permitted.
func CanPaymentTransition(from, to string) bool {
	for _, s := range PaymentTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// PaymentCodePrefix mirrors generatePaymentCode ("PAY-{booking_code}-XXXX").
const PaymentCodePrefix = "PAY-"

// Payment mirrors the payments table. Amount is integer cents.
type Payment struct {
	ID            uint64
	InvoiceID     uint64
	PaymentCode   string
	PaymentMethod string
	AmountCents   int64
	PaidAt        *time.Time
	Status        string
	ProofImage    *string // storage key, never exposed raw
	CustomerNote  *string
	AdminNote     *string
	VerifiedBy    *uint64
	VerifiedAt    *time.Time
	CreatedAt     *time.Time
	UpdatedAt     *time.Time
	// Loaded relations
	Invoice *Invoice
}
