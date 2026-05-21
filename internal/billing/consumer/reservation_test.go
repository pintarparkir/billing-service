package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/farid/billing-service/internal/billing/consumer"
	"github.com/farid/billing-service/internal/billing/model"
	mockuc "github.com/farid/billing-service/internal/billing/usecase/mock"
)

// ── reservation.created.v1 routing ────────────────────────────────────────

func TestHandle_Created_HappyPath(t *testing.T) {
	ctx := context.Background()
	uc := new(mockuc.MockBillingUsecase)
	// idem == reservation_id ensures one invoice per reservation on replay.
	uc.On("OpenInvoice", ctx, "res-123", "drv-1", "res-123").
		Return(&model.Invoice{ID: "inv-1"}, nil)

	c := consumer.NewReservation(uc)
	err := c.Handle(ctx, model.EvtReservationCreated, []byte(`{"reservation_id":"res-123","driver_id":"drv-1"}`))

	require.NoError(t, err)
	uc.AssertExpectations(t)
}

func TestHandle_Created_BadJSON_Drops(t *testing.T) {
	ctx := context.Background()
	uc := new(mockuc.MockBillingUsecase)

	c := consumer.NewReservation(uc)
	err := c.Handle(ctx, model.EvtReservationCreated, []byte("notjson"))

	require.NoError(t, err)
	uc.AssertNotCalled(t, "OpenInvoice", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestHandle_Created_MissingReservationID_Drops(t *testing.T) {
	ctx := context.Background()
	uc := new(mockuc.MockBillingUsecase)

	c := consumer.NewReservation(uc)
	err := c.Handle(ctx, model.EvtReservationCreated, []byte(`{"driver_id":"drv-1"}`))

	require.NoError(t, err)
	uc.AssertNotCalled(t, "OpenInvoice", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestHandle_Created_UsecaseError_Propagates(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("postgres down")
	uc := new(mockuc.MockBillingUsecase)
	uc.On("OpenInvoice", ctx, "res-9", "drv-9", "res-9").Return(nil, boom)

	c := consumer.NewReservation(uc)
	err := c.Handle(ctx, model.EvtReservationCreated, []byte(`{"reservation_id":"res-9","driver_id":"drv-9"}`))

	require.ErrorIs(t, err, boom)
	uc.AssertExpectations(t)
}

func TestHandle_Created_IdempotentReplay(t *testing.T) {
	ctx := context.Background()
	uc := new(mockuc.MockBillingUsecase)
	existing := &model.Invoice{ID: "inv-existing"}
	// Replay: OpenInvoice called twice, both succeed (repo guarantees one row).
	uc.On("OpenInvoice", ctx, "res-replay", "drv-r", "res-replay").
		Return(existing, nil).Twice()

	c := consumer.NewReservation(uc)
	body := []byte(`{"reservation_id":"res-replay","driver_id":"drv-r"}`)
	require.NoError(t, c.Handle(ctx, model.EvtReservationCreated, body))
	require.NoError(t, c.Handle(ctx, model.EvtReservationCreated, body))

	uc.AssertExpectations(t)
}

// ── reservation.cancelled.v1 routing ─────────────────────────────────────

func TestHandle_Cancelled_AppliesCancelFee(t *testing.T) {
	ctx := context.Background()
	uc := new(mockuc.MockBillingUsecase)
	uc.On("ApplyCancelFee", ctx, "res-cancel", mock.Anything, mock.Anything).
		Return(nil)

	c := consumer.NewReservation(uc)
	err := c.Handle(ctx, model.EvtReservationCancelled, []byte(`{
		"reservation_id":"res-cancel",
		"reason":"driver_request",
		"confirmed_at":"2026-05-21T10:00:00Z",
		"cancelled_at":"2026-05-21T10:05:00Z"
	}`))

	require.NoError(t, err)
	uc.AssertExpectations(t)
}

// ── reservation.expired.v1 routing ──────────────────────────────────────

func TestHandle_Expired_AppliesNoShowFee(t *testing.T) {
	ctx := context.Background()
	uc := new(mockuc.MockBillingUsecase)
	uc.On("ApplyNoShowFee", ctx, "res-expired").Return(nil)

	c := consumer.NewReservation(uc)
	err := c.Handle(ctx, model.EvtReservationExpired, []byte(`{"reservation_id":"res-expired"}`))

	require.NoError(t, err)
	uc.AssertExpectations(t)
}

// ── reservation.checked_out.v1 routing ──────────────────────────────────

func TestHandle_CheckedOut_ClosesInvoice(t *testing.T) {
	ctx := context.Background()
	uc := new(mockuc.MockBillingUsecase)
	inv := &model.Invoice{ID: "inv-co"}
	uc.On("GetInvoiceByReservation", ctx, "res-co").Return(inv, nil)
	uc.On("CloseInvoice", ctx, "inv-co", mock.Anything).Return(inv, nil)

	c := consumer.NewReservation(uc)
	err := c.Handle(ctx, model.EvtReservationCheckedOut, []byte(`{
		"reservation_id":"res-co",
		"confirmed_at":"2026-05-21T10:00:00Z",
		"checked_in_at":"2026-05-21T10:05:00Z",
		"checked_out_at":"2026-05-21T11:05:00Z"
	}`))

	require.NoError(t, err)
	uc.AssertExpectations(t)
}
