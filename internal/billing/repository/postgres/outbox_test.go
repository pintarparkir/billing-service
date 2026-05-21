package postgres_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/farid/billing-service/internal/billing/repository/postgres"
)

// ── FetchUnpublished ─────────────────────────────────────────────────────────

func TestOutboxRepo_FetchUnpublished_HappyPath(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := postgres.NewOutboxRepository(db)

	mock.ExpectQuery(`SELECT id, event_type, payload`).
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "event_type", "payload"}).
			AddRow(int64(1), "invoice.opened.v1", []byte(`{"invoice_id":"inv-1"}`)).
			AddRow(int64(2), "invoice.closed.v1", []byte(`{"invoice_id":"inv-2"}`)))

	rows, err := repo.FetchUnpublished(ctx, 10)

	require.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, int64(1), rows[0].ID)
	assert.Equal(t, "invoice.opened.v1", rows[0].EventType)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOutboxRepo_FetchUnpublished_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := postgres.NewOutboxRepository(db)

	mock.ExpectQuery(`SELECT id, event_type, payload`).
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "event_type", "payload"}))

	rows, err := repo.FetchUnpublished(ctx, 10)

	require.NoError(t, err)
	assert.Empty(t, rows)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── MarkPublished ────────────────────────────────────────────────────────────

func TestOutboxRepo_MarkPublished_HappyPath(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := postgres.NewOutboxRepository(db)

	ids := []int64{1, 2, 3}
	mock.ExpectExec(`UPDATE outbox_event SET published_at`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 3))

	err := repo.MarkPublished(ctx, ids)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOutboxRepo_MarkPublished_EmptySlice(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := postgres.NewOutboxRepository(db)

	err := repo.MarkPublished(ctx, []int64{})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
