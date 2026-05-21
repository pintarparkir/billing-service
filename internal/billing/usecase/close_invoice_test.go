package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/farid/billing-service/internal/billing/model"
	mockrepo "github.com/farid/billing-service/internal/billing/repository/mock"
	"github.com/farid/billing-service/pkg/pricing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// jakarta is a reference to Asia/Jakarta used across close/event tests.
var jakarta = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		panic(err)
	}
	return loc
}()

func TestCloseInvoice_HappyPath_HourlySession(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	inv := mkInvoice("inv-close-1", "res-1", model.InvoiceClosed)

	// 1h session → engine emits BOOKING (5k) + HOURLY (5k). No 0-IDR CANCELLATION.
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, jakarta)
	session := pricing.Session{
		ConfirmedAt:  now,
		CheckedInAt:  now,
		CheckedOutAt: now.Add(time.Hour),
		VehicleType:  pricing.VehicleCar,
		Timezone:     jakarta,
	}

	repo.On("GetByID", ctx, "inv-close-1").Return(inv, nil)
	repo.On("Close", ctx, "inv-close-1", mock.Anything, mock.Anything).Return(inv, nil)

	uc := newUC(repo)
	got, err := uc.CloseInvoice(ctx, "inv-close-1", session)

	require.NoError(t, err)
	assert.Equal(t, inv.ID, got.ID)
	repo.AssertExpectations(t)
}

func TestCloseInvoice_FiltersZeroCancellationLine(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	inv := mkInvoice("inv-close-2", "res-2", model.InvoiceClosed)

	// Grace cancel (1 min after confirm) → engine emits {LineCancellation, 0 IDR}.
	// That line must be stripped before calling repo.Close.
	confirm := time.Date(2026, 5, 20, 10, 0, 0, 0, jakarta)
	cancel := confirm.Add(1 * time.Minute)
	session := pricing.Session{
		ConfirmedAt: confirm,
		CancelledAt: &cancel,
		VehicleType: pricing.VehicleCar,
		Timezone:    jakarta,
	}

	repo.On("GetByID", ctx, "inv-close-2").Return(inv, nil)
	repo.On("Close", ctx, "inv-close-2",
		mock.MatchedBy(func(lines []model.LineItem) bool {
			for _, l := range lines {
				if l.Kind == model.LineCancellation && l.AmountIDR == 0 {
					return false
				}
			}
			return true
		}),
		mock.Anything,
	).Return(inv, nil)

	uc := newUC(repo)
	got, err := uc.CloseInvoice(ctx, "inv-close-2", session)

	require.NoError(t, err)
	assert.Equal(t, inv.ID, got.ID)
	repo.AssertExpectations(t)
}

func TestCloseInvoice_PostGraceCancelLine_NotFiltered(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	inv := mkInvoice("inv-close-3", "res-3", model.InvoiceClosed)

	// Post-grace cancel (10 min after confirm) → engine emits {LineCancellation, 5000}.
	// That line must survive the filter.
	confirm := time.Date(2026, 5, 20, 10, 0, 0, 0, jakarta)
	cancel := confirm.Add(10 * time.Minute)
	session := pricing.Session{
		ConfirmedAt: confirm,
		CancelledAt: &cancel,
		VehicleType: pricing.VehicleCar,
		Timezone:    jakarta,
	}

	repo.On("GetByID", ctx, "inv-close-3").Return(inv, nil)
	repo.On("Close", ctx, "inv-close-3",
		mock.MatchedBy(func(lines []model.LineItem) bool {
			for _, l := range lines {
				if l.Kind == model.LineCancellation && l.AmountIDR > 0 {
					return true
				}
			}
			return false
		}),
		mock.Anything,
	).Return(inv, nil)

	uc := newUC(repo)
	got, err := uc.CloseInvoice(ctx, "inv-close-3", session)

	require.NoError(t, err)
	assert.Equal(t, inv.ID, got.ID)
	repo.AssertExpectations(t)
}
