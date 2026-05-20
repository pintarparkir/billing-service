package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/farid/billing-service/internal/billing/model"
	mockrepo "github.com/farid/billing-service/internal/billing/repository/mock"
	"github.com/farid/billing-service/internal/billing/usecase"
	apperror "github.com/farid/billing-service/pkg/error"
	"github.com/farid/billing-service/pkg/pricing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newUC wires a BillingUsecase with the given repo mock and DefaultConfig.
func newUC(repo *mockrepo.MockInvoiceRepository) usecase.BillingUsecase {
	return usecase.NewBillingUsecase(
		repo,
		pricing.NewDefaultEngine(pricing.DefaultConfig()),
		pricing.DefaultConfig(),
	)
}

// mkInvoice returns a minimal Invoice for mock return values.
func mkInvoice(id, reservationID string, status model.InvoiceStatus) *model.Invoice {
	return &model.Invoice{
		ID:            id,
		ReservationID: reservationID,
		Status:        status,
		CreatedAt:     time.Now().UTC(),
	}
}

func TestOpenInvoice_MissingReservationID(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)

	uc := newUC(repo)
	_, err := uc.OpenInvoice(ctx, "", "driver-1", "idem-1")

	require.Error(t, err)
	var ae *apperror.AppError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, "VALIDATION", ae.Code)
	repo.AssertNotCalled(t, "Open")
}

func TestOpenInvoice_MissingIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)

	uc := newUC(repo)
	_, err := uc.OpenInvoice(ctx, "res-1", "driver-1", "")

	require.Error(t, err)
	var ae *apperror.AppError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, "VALIDATION", ae.Code)
	repo.AssertNotCalled(t, "Open")
}

func TestOpenInvoice_HappyPath(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	inv := mkInvoice("inv-1", "res-1", model.InvoiceOpen)

	// cfg.BookingFeeIDR = 5000 from DefaultConfig.
	repo.On("Open", ctx, "res-1", "driver-1", "idem-1", int64(5000), mock.Anything).
		Return(inv, nil)

	uc := newUC(repo)
	got, err := uc.OpenInvoice(ctx, "res-1", "driver-1", "idem-1")

	require.NoError(t, err)
	assert.Equal(t, inv.ID, got.ID)
	repo.AssertExpectations(t)
}
