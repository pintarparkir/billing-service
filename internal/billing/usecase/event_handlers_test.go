package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/farid/billing-service/internal/billing/model"
	mockrepo "github.com/farid/billing-service/internal/billing/repository/mock"
	apperror "github.com/farid/billing-service/pkg/error"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── ApplyCancelFee ──────────────────────────────────────────────────────────

func TestApplyCancelFee_NoInvoice_NoOp(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	// Reservation was cancelled before confirm → no invoice created yet.
	repo.On("GetByReservationID", ctx, "res-nocancel").Return(nil, apperror.ErrNotFound)

	uc := newUC(repo)
	err := uc.ApplyCancelFee(ctx, "res-nocancel", time.Now(), time.Now())

	require.NoError(t, err)
	repo.AssertNotCalled(t, "AppendLine")
}

func TestApplyCancelFee_AlreadyClosed_NoOp(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	inv := mkInvoice("inv-c-1", "res-closed", model.InvoiceClosed)
	repo.On("GetByReservationID", ctx, "res-closed").Return(inv, nil)

	uc := newUC(repo)
	err := uc.ApplyCancelFee(ctx, "res-closed", time.Now(), time.Now())

	require.NoError(t, err)
	repo.AssertNotCalled(t, "AppendLine")
}

func TestApplyCancelFee_WithinGrace_NoFeeAppended(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	inv := mkInvoice("inv-c-2", "res-grace", model.InvoiceOpen)
	repo.On("GetByReservationID", ctx, "res-grace").Return(inv, nil)

	// Cancel 1 min after confirm → within 2-min grace window → engine emits
	// LineCancellation with AmountIDR=0 → ApplyCancelFee must NOT call AppendLine.
	confirm := time.Date(2026, 5, 20, 10, 0, 0, 0, jakarta)
	cancel := confirm.Add(1 * time.Minute)

	uc := newUC(repo)
	err := uc.ApplyCancelFee(ctx, "res-grace", confirm, cancel)

	require.NoError(t, err)
	repo.AssertNotCalled(t, "AppendLine")
}

func TestApplyCancelFee_PostGrace_AppendsCancelFee(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	inv := mkInvoice("inv-c-3", "res-postgrace", model.InvoiceOpen)
	repo.On("GetByReservationID", ctx, "res-postgrace").Return(inv, nil)
	repo.On("AppendLine", ctx, "inv-c-3",
		mock.MatchedBy(func(l model.LineItem) bool {
			// DefaultConfig.CancelFeeIDR = 5000
			return l.Kind == model.LineCancellation && l.AmountIDR == 5000
		}),
		mock.Anything,
	).Return(inv, nil)

	// Cancel 10 min after confirm → post-grace → CancelFeeIDR=5000 appended.
	confirm := time.Date(2026, 5, 20, 10, 0, 0, 0, jakarta)
	cancel := confirm.Add(10 * time.Minute)

	uc := newUC(repo)
	err := uc.ApplyCancelFee(ctx, "res-postgrace", confirm, cancel)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// ── ApplyNoShowFee ──────────────────────────────────────────────────────────

func TestApplyNoShowFee_NoInvoice_NoOp(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	repo.On("GetByReservationID", ctx, "res-noshow-missing").Return(nil, apperror.ErrNotFound)

	uc := newUC(repo)
	err := uc.ApplyNoShowFee(ctx, "res-noshow-missing")

	require.NoError(t, err)
	repo.AssertNotCalled(t, "AppendLine")
}

func TestApplyNoShowFee_AlreadyClosed_NoOp(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	inv := mkInvoice("inv-ns-1", "res-ns-closed", model.InvoiceClosed)
	repo.On("GetByReservationID", ctx, "res-ns-closed").Return(inv, nil)

	uc := newUC(repo)
	err := uc.ApplyNoShowFee(ctx, "res-ns-closed")

	require.NoError(t, err)
	repo.AssertNotCalled(t, "AppendLine")
}

func TestApplyNoShowFee_HappyPath_AppendsNoShowLine(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	inv := mkInvoice("inv-ns-2", "res-noshow", model.InvoiceOpen)
	repo.On("GetByReservationID", ctx, "res-noshow").Return(inv, nil)
	repo.On("AppendLine", ctx, "inv-ns-2",
		mock.MatchedBy(func(l model.LineItem) bool {
			// DefaultConfig.NoShowFeeIDR = 5000
			return l.Kind == model.LineNoShow && l.AmountIDR == 5000
		}),
		mock.Anything,
	).Return(inv, nil)

	uc := newUC(repo)
	err := uc.ApplyNoShowFee(ctx, "res-noshow")

	require.NoError(t, err)
	repo.AssertExpectations(t)
}
