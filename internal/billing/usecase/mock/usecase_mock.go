// Package mockusecase provides a testify/mock implementation of
// usecase.BillingUsecase for unit tests that exercise consumers or other
// callers without a real billing pipeline.
package mockusecase

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/farid/billing-service/internal/billing/model"
	"github.com/farid/billing-service/internal/billing/usecase"
	"github.com/farid/billing-service/pkg/pricing"
)

type MockBillingUsecase struct {
	mock.Mock
}

var _ usecase.BillingUsecase = (*MockBillingUsecase)(nil)

func (m *MockBillingUsecase) OpenInvoice(ctx context.Context, reservationID, driverID, idem string) (*model.Invoice, error) {
	args := m.Called(ctx, reservationID, driverID, idem)
	inv, _ := args.Get(0).(*model.Invoice)
	return inv, args.Error(1)
}

func (m *MockBillingUsecase) CloseInvoice(ctx context.Context, invoiceID string, session pricing.Session) (*model.Invoice, error) {
	args := m.Called(ctx, invoiceID, session)
	inv, _ := args.Get(0).(*model.Invoice)
	return inv, args.Error(1)
}

func (m *MockBillingUsecase) GetInvoice(ctx context.Context, id string) (*model.Invoice, error) {
	args := m.Called(ctx, id)
	inv, _ := args.Get(0).(*model.Invoice)
	return inv, args.Error(1)
}

func (m *MockBillingUsecase) GetInvoiceByReservation(ctx context.Context, reservationID string) (*model.Invoice, error) {
	args := m.Called(ctx, reservationID)
	inv, _ := args.Get(0).(*model.Invoice)
	return inv, args.Error(1)
}

func (m *MockBillingUsecase) ApplyCancelFee(ctx context.Context, reservationID string, confirmedAt, cancelledAt time.Time) error {
	return m.Called(ctx, reservationID, confirmedAt, cancelledAt).Error(0)
}

func (m *MockBillingUsecase) ApplyNoShowFee(ctx context.Context, reservationID string) error {
	return m.Called(ctx, reservationID).Error(0)
}
