package postgres_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/farid/billing-service/internal/billing/model"
	"github.com/farid/billing-service/internal/billing/repository/postgres"
	apperror "github.com/farid/billing-service/pkg/error"
)

func newMockDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return sqlx.NewDb(db, "postgres"), mock
}

// ── Open ─────────────────────────────────────────────────────────────────────

func TestInvoiceRepo_Open_HappyPath(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := postgres.NewInvoiceRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO invoice`).
		WithArgs("res-1", "drv-1", int64(5000), "idem-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
			AddRow("inv-1", time.Now()))
	mock.ExpectExec(`INSERT INTO invoice_line`).
		WithArgs("inv-1", model.LineBooking, int64(5000), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO outbox_event`).
		WithArgs("inv-1", model.EvtInvoiceOpened, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// GetByID after commit
	mock.ExpectQuery(`SELECT id, reservation_id`).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "reservation_id", "driver_id", "status", "total_idr",
			"idempotency_key", "created_at", "closed_at", "paid_at",
		}).AddRow("inv-1", "res-1", "drv-1", "OPEN", int64(5000), "idem-1",
			time.Now(), nil, nil))
	mock.ExpectQuery(`SELECT id, invoice_id, kind, amount_idr`).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "invoice_id", "kind", "amount_idr", "metadata", "created_at",
		}).AddRow("line-1", "inv-1", "BOOKING", int64(5000), []byte(`{}`), time.Now()))

	inv, err := repo.Open(ctx, "res-1", "drv-1", "idem-1", 5000, []byte(`{}`))

	require.NoError(t, err)
	assert.Equal(t, "inv-1", inv.ID)
	assert.Equal(t, "res-1", inv.ReservationID)
	assert.Equal(t, model.InvoiceOpen, inv.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInvoiceRepo_Open_IdempotentReplay(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := postgres.NewInvoiceRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO invoice`).
		WithArgs("res-1", "drv-1", int64(5000), "idem-dup").
		WillReturnError(&pq.Error{Code: "23505"})
	mock.ExpectRollback()

	// findByIdem fallback
	mock.ExpectQuery(`SELECT id FROM invoice WHERE idempotency_key`).
		WithArgs("idem-dup").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("inv-existing"))

	// GetByID
	mock.ExpectQuery(`SELECT id, reservation_id`).
		WithArgs("inv-existing").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "reservation_id", "driver_id", "status", "total_idr",
			"idempotency_key", "created_at", "closed_at", "paid_at",
		}).AddRow("inv-existing", "res-1", "drv-1", "OPEN", int64(5000), "idem-dup",
			time.Now(), nil, nil))
	mock.ExpectQuery(`SELECT id, invoice_id, kind`).
		WithArgs("inv-existing").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "invoice_id", "kind", "amount_idr", "metadata", "created_at",
		}))

	inv, err := repo.Open(ctx, "res-1", "drv-1", "idem-dup", 5000, []byte(`{}`))

	require.NoError(t, err)
	assert.Equal(t, "inv-existing", inv.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── GetByID ──────────────────────────────────────────────────────────────────

func TestInvoiceRepo_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := postgres.NewInvoiceRepository(db)

	mock.ExpectQuery(`SELECT id, reservation_id`).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "reservation_id", "driver_id", "status", "total_idr",
			"idempotency_key", "created_at", "closed_at", "paid_at",
		}).AddRow("inv-1", "res-1", "drv-1", "OPEN", int64(5000), "idem-1",
			time.Now(), nil, nil))
	mock.ExpectQuery(`SELECT id, invoice_id, kind`).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "invoice_id", "kind", "amount_idr", "metadata", "created_at",
		}))

	inv, err := repo.GetByID(ctx, "inv-1")

	require.NoError(t, err)
	assert.Equal(t, "inv-1", inv.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInvoiceRepo_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := postgres.NewInvoiceRepository(db)

	mock.ExpectQuery(`SELECT id, reservation_id`).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.GetByID(ctx, "missing")

	require.Error(t, err)
	assert.True(t, apperror.Is(err, apperror.ErrNotFound))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── GetByReservationID ───────────────────────────────────────────────────────

func TestInvoiceRepo_GetByReservationID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := postgres.NewInvoiceRepository(db)

	mock.ExpectQuery(`SELECT id FROM invoice WHERE reservation_id`).
		WithArgs("res-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("inv-1"))
	mock.ExpectQuery(`SELECT id, reservation_id`).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "reservation_id", "driver_id", "status", "total_idr",
			"idempotency_key", "created_at", "closed_at", "paid_at",
		}).AddRow("inv-1", "res-1", "drv-1", "OPEN", int64(5000), "idem-1",
			time.Now(), nil, nil))
	mock.ExpectQuery(`SELECT id, invoice_id, kind`).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "invoice_id", "kind", "amount_idr", "metadata", "created_at",
		}))

	inv, err := repo.GetByReservationID(ctx, "res-1")

	require.NoError(t, err)
	assert.Equal(t, "inv-1", inv.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInvoiceRepo_GetByReservationID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := postgres.NewInvoiceRepository(db)

	mock.ExpectQuery(`SELECT id FROM invoice WHERE reservation_id`).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.GetByReservationID(ctx, "missing")

	require.Error(t, err)
	assert.True(t, apperror.Is(err, apperror.ErrNotFound))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Close ────────────────────────────────────────────────────────────────────

func TestInvoiceRepo_Close_HappyPath(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := postgres.NewInvoiceRepository(db)

	lines := []model.LineItem{
		{Kind: model.LineHourly, AmountIDR: 10000, Metadata: map[string]any{"hours": 2}},
	}
	evt, _ := json.Marshal(map[string]any{"invoice_id": "inv-1"})

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status, total_idr FROM invoice WHERE id`).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{"status", "total_idr"}).
			AddRow("OPEN", int64(5000)))
	mock.ExpectExec(`INSERT INTO invoice_line`).
		WithArgs("inv-1", model.LineHourly, int64(10000), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE invoice`).
		WithArgs("inv-1", int64(15000)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO outbox_event`).
		WithArgs("inv-1", model.EvtInvoiceClosed, evt).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// GetByID after commit
	mock.ExpectQuery(`SELECT id, reservation_id`).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "reservation_id", "driver_id", "status", "total_idr",
			"idempotency_key", "created_at", "closed_at", "paid_at",
		}).AddRow("inv-1", "res-1", "drv-1", "CLOSED", int64(15000), "idem-1",
			time.Now(), time.Now(), nil))
	mock.ExpectQuery(`SELECT id, invoice_id, kind`).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "invoice_id", "kind", "amount_idr", "metadata", "created_at",
		}))

	inv, err := repo.Close(ctx, "inv-1", lines, evt)

	require.NoError(t, err)
	assert.Equal(t, "inv-1", inv.ID)
	assert.Equal(t, model.InvoiceClosed, inv.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInvoiceRepo_Close_AlreadyClosed(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := postgres.NewInvoiceRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status, total_idr FROM invoice WHERE id`).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{"status", "total_idr"}).
			AddRow("CLOSED", int64(15000)))
	mock.ExpectRollback()

	// GetByID fallback
	mock.ExpectQuery(`SELECT id, reservation_id`).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "reservation_id", "driver_id", "status", "total_idr",
			"idempotency_key", "created_at", "closed_at", "paid_at",
		}).AddRow("inv-1", "res-1", "drv-1", "CLOSED", int64(15000), "idem-1",
			time.Now(), time.Now(), nil))
	mock.ExpectQuery(`SELECT id, invoice_id, kind`).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "invoice_id", "kind", "amount_idr", "metadata", "created_at",
		}))

	inv, err := repo.Close(ctx, "inv-1", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, model.InvoiceClosed, inv.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── AppendLine ───────────────────────────────────────────────────────────────

func TestInvoiceRepo_AppendLine_HappyPath(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := postgres.NewInvoiceRepository(db)

	line := model.LineItem{Kind: model.LineCancellation, AmountIDR: 2000, Metadata: map[string]any{"reason": "cancel"}}
	evt, _ := json.Marshal(map[string]any{"invoice_id": "inv-1"})

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO invoice_line`).
		WithArgs("inv-1", model.LineCancellation, int64(2000), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE invoice SET total_idr`).
		WithArgs("inv-1", int64(2000)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO outbox_event`).
		WithArgs("inv-1", model.EvtInvoiceClosed, evt).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// GetByID after commit
	mock.ExpectQuery(`SELECT id, reservation_id`).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "reservation_id", "driver_id", "status", "total_idr",
			"idempotency_key", "created_at", "closed_at", "paid_at",
		}).AddRow("inv-1", "res-1", "drv-1", "OPEN", int64(7000), "idem-1",
			time.Now(), nil, nil))
	mock.ExpectQuery(`SELECT id, invoice_id, kind`).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "invoice_id", "kind", "amount_idr", "metadata", "created_at",
		}))

	inv, err := repo.AppendLine(ctx, "inv-1", line, evt)

	require.NoError(t, err)
	assert.Equal(t, "inv-1", inv.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}
