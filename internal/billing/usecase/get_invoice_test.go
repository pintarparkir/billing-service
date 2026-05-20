package usecase_test

import (
	"context"
	"testing"

	"github.com/farid/billing-service/internal/billing/model"
	mockrepo "github.com/farid/billing-service/internal/billing/repository/mock"
	apperror "github.com/farid/billing-service/pkg/error"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetInvoice_OK(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	inv := mkInvoice("inv-get-1", "res-1", model.InvoiceOpen)
	repo.On("GetByID", ctx, "inv-get-1").Return(inv, nil)

	uc := newUC(repo)
	got, err := uc.GetInvoice(ctx, "inv-get-1")

	require.NoError(t, err)
	assert.Equal(t, inv, got)
	repo.AssertExpectations(t)
}

func TestGetInvoice_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	repo.On("GetByID", ctx, "missing").Return(nil, apperror.ErrNotFound)

	uc := newUC(repo)
	_, err := uc.GetInvoice(ctx, "missing")

	require.Error(t, err)
	assert.True(t, apperror.Is(err, apperror.ErrNotFound))
}

func TestGetInvoiceByReservation_OK(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	inv := mkInvoice("inv-get-2", "res-42", model.InvoiceClosed)
	repo.On("GetByReservationID", ctx, "res-42").Return(inv, nil)

	uc := newUC(repo)
	got, err := uc.GetInvoiceByReservation(ctx, "res-42")

	require.NoError(t, err)
	assert.Equal(t, inv, got)
	repo.AssertExpectations(t)
}

func TestGetInvoiceByReservation_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	repo.On("GetByReservationID", ctx, "res-missing").Return(nil, apperror.ErrNotFound)

	uc := newUC(repo)
	_, err := uc.GetInvoiceByReservation(ctx, "res-missing")

	require.Error(t, err)
	assert.True(t, apperror.Is(err, apperror.ErrNotFound))
}
