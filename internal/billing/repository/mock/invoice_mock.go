package mockrepo

import (
	"context"

	"github.com/farid/billing-service/internal/billing/model"
	"github.com/stretchr/testify/mock"
)

// MockInvoiceRepository implements repository.InvoiceRepository.
type MockInvoiceRepository struct {
	mock.Mock
}

func (m *MockInvoiceRepository) Open(
	ctx context.Context,
	reservationID, driverID, idempotencyKey string,
	bookingFeeIDR int64,
	eventPayload []byte,
) (*model.Invoice, error) {
	args := m.Called(ctx, reservationID, driverID, idempotencyKey, bookingFeeIDR, eventPayload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Invoice), args.Error(1)
}

func (m *MockInvoiceRepository) Close(
	ctx context.Context,
	invoiceID string,
	lines []model.LineItem,
	eventPayload []byte,
) (*model.Invoice, error) {
	args := m.Called(ctx, invoiceID, lines, eventPayload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Invoice), args.Error(1)
}

func (m *MockInvoiceRepository) GetByID(ctx context.Context, id string) (*model.Invoice, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Invoice), args.Error(1)
}

func (m *MockInvoiceRepository) GetByReservationID(ctx context.Context, reservationID string) (*model.Invoice, error) {
	args := m.Called(ctx, reservationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Invoice), args.Error(1)
}

func (m *MockInvoiceRepository) AppendLine(
	ctx context.Context,
	invoiceID string,
	line model.LineItem,
	eventPayload []byte,
) (*model.Invoice, error) {
	args := m.Called(ctx, invoiceID, line, eventPayload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Invoice), args.Error(1)
}
