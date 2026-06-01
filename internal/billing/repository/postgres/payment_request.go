// Package postgres implements payment request repository using PostgreSQL.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/farid/billing-service/internal/billing/model"
	"github.com/farid/billing-service/internal/billing/repository"
	apperror "github.com/farid/billing-service/pkg/error"
)

type paymentRequestRepo struct{ db *sqlx.DB }

func NewPaymentRequestRepository(db *sqlx.DB) repository.PaymentRequestRepository {
	return &paymentRequestRepo{db: db}
}

const insertPaymentRequestSQL = `
INSERT INTO payment_requests (id, reservation_id, invoice_id, amount_idr, method, status, payment_ref, qris_url, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING created_at, updated_at
`

func (r *paymentRequestRepo) Create(ctx context.Context, req *model.PaymentRequest) (*model.PaymentRequest, error) {
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.PaymentRef == "" {
		req.PaymentRef = fmt.Sprintf("PAY-%s", req.ID)
	}

	var createdAt, updatedAt pq.NullTime
	err := r.db.QueryRowxContext(ctx, insertPaymentRequestSQL,
		req.ID,
		req.ReservationID,
		req.InvoiceID,
		req.AmountIDR,
		string(req.Method),
		string(req.Status),
		req.PaymentRef,
		req.QRISURL,
		req.ExpiresAt,
	).Scan(&createdAt, &updatedAt)

	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && string(pgErr.Code) == codeUniqueViolation {
			return nil, &apperror.AppError{Code: "CONFLICT", Message: "payment request already exists"}
		}
		return nil, err
	}

	if createdAt.Valid {
		req.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		req.UpdatedAt = updatedAt.Time
	}

	return req, nil
}

const getPaymentRequestByIDSQL = `
SELECT id, reservation_id, invoice_id, amount_idr, method, status, payment_ref, qris_url, expires_at, created_at, updated_at
FROM payment_requests
WHERE id = $1
`

func (r *paymentRequestRepo) GetByID(ctx context.Context, id string) (*model.PaymentRequest, error) {
	var row paymentRequestRow
	err := r.db.QueryRowxContext(ctx, getPaymentRequestByIDSQL, id).StructScan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toModel(), nil
}

const getPaymentRequestByReservationSQL = `
SELECT id, reservation_id, invoice_id, amount_idr, method, status, payment_ref, qris_url, expires_at, created_at, updated_at
FROM payment_requests
WHERE reservation_id = $1
ORDER BY created_at DESC
LIMIT 1
`

func (r *paymentRequestRepo) GetByReservationID(ctx context.Context, reservationID string) (*model.PaymentRequest, error) {
	var row paymentRequestRow
	err := r.db.QueryRowxContext(ctx, getPaymentRequestByReservationSQL, reservationID).StructScan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toModel(), nil
}

const updatePaymentRequestStatusSQL = `
UPDATE payment_requests
SET status = $2, updated_at = now()
WHERE id = $1
`

func (r *paymentRequestRepo) UpdateStatus(ctx context.Context, id string, status model.PaymentRequestStatus) error {
	_, err := r.db.ExecContext(ctx, updatePaymentRequestStatusSQL, id, string(status))
	return err
}

type paymentRequestRow struct {
	ID            string      `db:"id"`
	ReservationID string      `db:"reservation_id"`
	InvoiceID     string      `db:"invoice_id"`
	AmountIDR     int64       `db:"amount_idr"`
	Method        string      `db:"method"`
	Status        string      `db:"status"`
	PaymentRef    string      `db:"payment_ref"`
	QRISURL       string      `db:"qris_url"`
	ExpiresAt     pq.NullTime `db:"expires_at"`
	CreatedAt     pq.NullTime `db:"created_at"`
	UpdatedAt     pq.NullTime `db:"updated_at"`
}

func (r paymentRequestRow) toModel() *model.PaymentRequest {
	out := &model.PaymentRequest{
		ID:            r.ID,
		ReservationID: r.ReservationID,
		InvoiceID:     r.InvoiceID,
		AmountIDR:     r.AmountIDR,
		Method:        model.PaymentMethod(r.Method),
		Status:        model.PaymentRequestStatus(r.Status),
		PaymentRef:    r.PaymentRef,
		QRISURL:       r.QRISURL,
	}
	if r.ExpiresAt.Valid {
		out.ExpiresAt = r.ExpiresAt.Time
	}
	if r.CreatedAt.Valid {
		out.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		out.UpdatedAt = r.UpdatedAt.Time
	}
	return out
}
