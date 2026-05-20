package model

// gRPC method full names for the idempotency interceptor.
// Keep in sync with proto in api/proto/billing/v1/billing.proto.
const (
	SCOPE_OPEN_INVOICE  = "/parkirpintar.billing.v1.BillingService/OpenInvoice"
	SCOPE_CLOSE_INVOICE = "/parkirpintar.billing.v1.BillingService/CloseInvoice"
)

// Routing keys we publish on parkirpintar.events.
const (
	EvtInvoiceOpened = "billing.invoice.opened.v1"
	EvtInvoiceClosed = "billing.invoice.closed.v1"
)

// Routing keys we subscribe to (from reservation-service).
const (
	EvtReservationCreated    = "reservation.created.v1"
	EvtReservationCheckedOut = "reservation.checked_out.v1"
	EvtReservationCancelled  = "reservation.cancelled.v1"
	EvtReservationExpired    = "reservation.expired.v1"
)
