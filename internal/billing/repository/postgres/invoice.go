// Package postgres implements invoice repository using PostgreSQL.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/farid/billing-service/internal/billing/model"
	"github.com/farid/billing-service/internal/billing/repository"
	apperror "github.com/farid/billing-service/pkg/error"
)

const codeUniqueViolation = "23505"

type invoiceRepo struct{ db *sqlx.DB }

func NewInvoiceRepository(db *sqlx.DB) repository.InvoiceRepository {
	return &invoiceRepo{db: db}
}

const insertInvoiceSQL = `
INSERT INTO invoice (reservation_id, driver_id, status, total_idr, idempotency_key)
VALUES ($1, $2, 'OPEN', $3, $4)
RETURNING id, created_at
`

const insertLineSQL = `
INSERT INTO invoice_line (invoice_id, kind, amount_idr, metadata)
VALUES ($1, $2, $3, $4)
`

const insertOutboxSQL = `
INSERT INTO outbox_event (aggregate_type, aggregate_id, event_type, payload)
VALUES ('invoice', $1, $2, $3)
`

func (r *invoiceRepo) Open(ctx context.Context, reservationID, driverID, idem string, bookingFee int64, evt []byte) (*model.Invoice, error) {
	// Idempotent on idempotency_key. Sequence:
	//   1. Try INSERT; on UNIQUE-violation, SELECT existing.
	//   2. If we inserted, also INSERT booking line + outbox row; commit.
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var id string
	var createdAt sql.NullTime
	err = tx.QueryRowxContext(ctx, insertInvoiceSQL, reservationID, driverID, bookingFee, idem).Scan(&id, &createdAt)
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && string(pgErr.Code) == codeUniqueViolation {
			// Replay path: return the existing row, no extra writes.
			_ = tx.Rollback()
			return r.findByIdem(ctx, idem)
		}
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, insertLineSQL, id, model.LineBooking, bookingFee, []byte(`{}`)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, insertOutboxSQL, id, model.EvtInvoiceOpened, evt); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.GetByID(ctx, id)
}

const findByIdemSQL = `SELECT id FROM invoice WHERE idempotency_key = $1`

func (r *invoiceRepo) findByIdem(ctx context.Context, idem string) (*model.Invoice, error) {
	var id string
	err := r.db.QueryRowxContext(ctx, findByIdemSQL, idem).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

const getByIDSQL = `
SELECT id, reservation_id, driver_id::text AS driver_id, status, total_idr,
       coalesce(idempotency_key,'') AS idempotency_key,
       created_at, closed_at, paid_at
FROM invoice WHERE id = $1
`

const getLinesSQL = `
SELECT id, invoice_id, kind, amount_idr, metadata, created_at
FROM invoice_line WHERE invoice_id = $1 ORDER BY created_at, id
`

func (r *invoiceRepo) GetByID(ctx context.Context, id string) (*model.Invoice, error) {
	var iv invoiceRow
	if err := r.db.QueryRowxContext(ctx, getByIDSQL, id).StructScan(&iv); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, err
	}
	out := iv.toModel()

	lines, err := r.fetchLines(ctx, id)
	if err != nil {
		return nil, err
	}
	out.LineItems = lines
	return out, nil
}

func (r *invoiceRepo) GetByReservationID(ctx context.Context, reservationID string) (*model.Invoice, error) {
	var id string
	err := r.db.QueryRowxContext(ctx,
		`SELECT id FROM invoice WHERE reservation_id = $1`, reservationID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *invoiceRepo) fetchLines(ctx context.Context, invoiceID string) ([]model.LineItem, error) {
	rows, err := r.db.QueryxContext(ctx, getLinesSQL, invoiceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []model.LineItem
	for rows.Next() {
		var lr lineRow
		if err := rows.StructScan(&lr); err != nil {
			return nil, err
		}
		out = append(out, lr.toModel())
	}
	return out, rows.Err()
}

const closeInvoiceSQL = `
UPDATE invoice
   SET status = 'CLOSED', total_idr = $2, closed_at = now()
 WHERE id = $1 AND status = 'OPEN'
`

func (r *invoiceRepo) Close(ctx context.Context, invoiceID string, lines []model.LineItem, evt []byte) (*model.Invoice, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	var existingTotal int64
	err = tx.QueryRowxContext(ctx,
		`SELECT status, total_idr FROM invoice WHERE id = $1 FOR UPDATE`, invoiceID,
	).Scan(&status, &existingTotal)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != string(model.InvoiceOpen) {
		// Already closed → idempotent no-op.
		_ = tx.Rollback()
		return r.GetByID(ctx, invoiceID)
	}

	newLineTotal := int64(0)
	for _, l := range lines {
		if l.Kind == model.LineBooking {
			continue // skip: booking-fee row already inserted on Open
		}
		if err := insertLine(ctx, tx, invoiceID, l); err != nil {
			return nil, err
		}
		newLineTotal += l.AmountIDR
	}

	finalTotal := existingTotal + newLineTotal
	if _, err := tx.ExecContext(ctx, closeInvoiceSQL, invoiceID, finalTotal); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, insertOutboxSQL, invoiceID, model.EvtInvoiceClosed, evt); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, invoiceID)
}

func (r *invoiceRepo) AppendLine(ctx context.Context, invoiceID string, line model.LineItem, evt []byte) (*model.Invoice, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertLine(ctx, tx, invoiceID, line); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE invoice SET total_idr = total_idr + $2 WHERE id = $1`,
		invoiceID, line.AmountIDR,
	); err != nil {
		return nil, err
	}
	if evt != nil {
		if _, err := tx.ExecContext(ctx, insertOutboxSQL, invoiceID, model.EvtInvoiceClosed, evt); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, invoiceID)
}

// insertLine marshals the line's metadata and inserts one invoice_line row.
func insertLine(ctx context.Context, tx *sqlx.Tx, invoiceID string, l model.LineItem) error {
	md := l.Metadata
	if md == nil {
		md = map[string]any{}
	}
	mdB, err := json.Marshal(md)
	if err != nil {
		return fmt.Errorf("billing: marshal line metadata: %w", err)
	}
	_, err = tx.ExecContext(ctx, insertLineSQL, invoiceID, l.Kind, l.AmountIDR, mdB)
	return err
}

// ── row types ────────────────────────────────────────────────────────────────

type invoiceRow struct {
	ID             string      `db:"id"`
	ReservationID  string      `db:"reservation_id"`
	DriverID       string      `db:"driver_id"`
	Status         string      `db:"status"`
	TotalIDR       int64       `db:"total_idr"`
	IdempotencyKey string      `db:"idempotency_key"`
	CreatedAt      pq.NullTime `db:"created_at"`
	ClosedAt       pq.NullTime `db:"closed_at"`
	PaidAt         pq.NullTime `db:"paid_at"`
}

func (r invoiceRow) toModel() *model.Invoice {
	out := &model.Invoice{
		ID:             r.ID,
		ReservationID:  r.ReservationID,
		DriverID:       r.DriverID,
		Status:         model.InvoiceStatus(r.Status),
		TotalIDR:       r.TotalIDR,
		IdempotencyKey: r.IdempotencyKey,
	}
	if r.CreatedAt.Valid {
		out.CreatedAt = r.CreatedAt.Time
	}
	if r.ClosedAt.Valid {
		t := r.ClosedAt.Time
		out.ClosedAt = &t
	}
	if r.PaidAt.Valid {
		t := r.PaidAt.Time
		out.PaidAt = &t
	}
	return out
}

type lineRow struct {
	ID        string      `db:"id"`
	InvoiceID string      `db:"invoice_id"`
	Kind      string      `db:"kind"`
	AmountIDR int64       `db:"amount_idr"`
	Metadata  []byte      `db:"metadata"`
	CreatedAt pq.NullTime `db:"created_at"`
}

func (l lineRow) toModel() model.LineItem {
	out := model.LineItem{
		ID:        l.ID,
		InvoiceID: l.InvoiceID,
		Kind:      model.LineKind(l.Kind),
		AmountIDR: l.AmountIDR,
	}
	if len(l.Metadata) > 0 {
		var m map[string]any
		if err := json.Unmarshal(l.Metadata, &m); err == nil {
			out.Metadata = m
		}
	}
	if l.CreatedAt.Valid {
		out.CreatedAt = l.CreatedAt.Time
	}
	return out
}
