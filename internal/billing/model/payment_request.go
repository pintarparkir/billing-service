package model

import "time"

type PaymentMethod string

const (
	PaymentMethodQRIS PaymentMethod = "QRIS"
	PaymentMethodCC   PaymentMethod = "CC"
)

type PaymentRequestStatus string

const (
	PaymentRequestPending PaymentRequestStatus = "PENDING"
	PaymentRequestSuccess PaymentRequestStatus = "SUCCESS"
	PaymentRequestFailed  PaymentRequestStatus = "FAILED"
	PaymentRequestExpired PaymentRequestStatus = "EXPIRED"
)

type PaymentRequest struct {
	ID            string
	ReservationID string
	InvoiceID     string
	AmountIDR     int64
	Method        PaymentMethod
	Status        PaymentRequestStatus
	PaymentRef    string
	QRISURL       string
	ExpiresAt     time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
