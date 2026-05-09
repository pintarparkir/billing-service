// Package repository defines persistence contracts for billing-service.
package repository

import (
	"context"

	"github.com/farid/billing-service/internal/billing/model"
)

// InvoiceRepository persists invoices and their line items.
// Open + outbox row, and Close + line inserts + outbox row, both happen in
// single transactions inside the postgres impl.
type InvoiceRepository interface {
	// Open creates an OPEN invoice with a single BOOKING line. Idempotent on
	// idempotency_key: re-calling with the same key returns the original row.
	Open(ctx context.Context, reservationID, driverID, idempotencyKey string, bookingFeeIDR int64, eventPayload []byte) (*model.Invoice, error)

	// Close transitions OPEN → CLOSED. Atomically:
	//   1. inserts the supplied LineItems,
	//   2. updates total_idr = SUM(lines) + previously-existing booking,
	//   3. sets status, closed_at,
	//   4. appends an outbox row.
	// Re-calling on an already-CLOSED invoice is a no-op (idempotent).
	Close(ctx context.Context, invoiceID string, lines []model.LineItem, eventPayload []byte) (*model.Invoice, error)

	GetByID(ctx context.Context, id string) (*model.Invoice, error)
	GetByReservationID(ctx context.Context, reservationID string) (*model.Invoice, error)

	// AppendLine adds a single one-off line to an OPEN invoice (used by the
	// cancel-fee / no-show-fee consumer). Returns the updated invoice.
	AppendLine(ctx context.Context, invoiceID string, line model.LineItem, eventPayload []byte) (*model.Invoice, error)
}

type OutboxRepository interface {
	FetchUnpublished(ctx context.Context, limit int) ([]OutboxRow, error)
	MarkPublished(ctx context.Context, ids []int64) error
}

type OutboxRow struct {
	ID        int64  `db:"id"`
	EventType string `db:"event_type"`
	Payload   []byte `db:"payload"`
}
