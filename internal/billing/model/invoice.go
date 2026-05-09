package model

import "time"

type InvoiceStatus string

const (
	InvoiceOpen   InvoiceStatus = "OPEN"
	InvoiceClosed InvoiceStatus = "CLOSED"
	InvoicePaid   InvoiceStatus = "PAID"
	InvoiceVoid   InvoiceStatus = "VOID"
)

type LineKind string

const (
	LineBooking      LineKind = "BOOKING"
	LineHourly       LineKind = "HOURLY"
	LineOvernight    LineKind = "OVERNIGHT"
	LineCancellation LineKind = "CANCELLATION"
	LineNoShow       LineKind = "NOSHOW"
)

type LineItem struct {
	ID        string
	InvoiceID string
	Kind      LineKind
	AmountIDR int64
	Metadata  map[string]any
	CreatedAt time.Time
}

type Invoice struct {
	ID             string
	ReservationID  string
	Status         InvoiceStatus
	LineItems      []LineItem
	TotalIDR       int64
	IdempotencyKey string
	CreatedAt      time.Time
	ClosedAt       *time.Time
	PaidAt         *time.Time
}
