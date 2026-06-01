package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/farid/billing-service/internal/billing/model"
	"github.com/farid/billing-service/internal/billing/repository"
	mockrepo "github.com/farid/billing-service/internal/billing/repository/mock"
	"github.com/farid/billing-service/internal/billing/usecase"
	apperror "github.com/farid/billing-service/pkg/error"
	"github.com/farid/billing-service/pkg/pricing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockPaymentRequestRepository struct {
	mock.Mock
}

func (m *mockPaymentRequestRepository) Create(ctx context.Context, req *model.PaymentRequest) (*model.PaymentRequest, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PaymentRequest), args.Error(1)
}

func (m *mockPaymentRequestRepository) GetByID(ctx context.Context, id string) (*model.PaymentRequest, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PaymentRequest), args.Error(1)
}

func (m *mockPaymentRequestRepository) GetByReservationID(ctx context.Context, reservationID string) (*model.PaymentRequest, error) {
	args := m.Called(ctx, reservationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PaymentRequest), args.Error(1)
}

func (m *mockPaymentRequestRepository) UpdateStatus(ctx context.Context, id string, status model.PaymentRequestStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *mockPaymentRequestRepository) GetByPaymentRef(ctx context.Context, paymentRef string) (*model.PaymentRequest, error) {
	args := m.Called(ctx, paymentRef)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PaymentRequest), args.Error(1)
}

func (m *mockPaymentRequestRepository) UpdateStatusByPaymentRef(ctx context.Context, paymentRef string, status model.PaymentRequestStatus, eventType string, eventPayload []byte) (*model.PaymentRequest, error) {
	args := m.Called(ctx, paymentRef, status, eventType, eventPayload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PaymentRequest), args.Error(1)
}

func newUCWithPaymentRepo(repo *mockrepo.MockInvoiceRepository, paymentRepo repository.PaymentRequestRepository) usecase.BillingUsecase {
	return usecase.NewBillingUsecase(
		repo,
		pricing.NewDefaultEngine(pricing.DefaultConfig()),
		pricing.DefaultConfig(),
	).WithPaymentRequestRepository(paymentRepo)
}

func TestCreatePaymentRequest_MissingReservationID(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	paymentRepo := new(mockPaymentRequestRepository)
	uc := newUCWithPaymentRepo(repo, paymentRepo)

	got, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		DriverID:  "driver-1",
		AmountIDR: 5000,
		Method:    "QRIS",
	})

	require.Error(t, err)
	assert.Nil(t, got)
	var ae *apperror.AppError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, "VALIDATION", ae.Code)
	repo.AssertNotCalled(t, "GetByReservationID")
	paymentRepo.AssertNotCalled(t, "Create")
}

func TestCreatePaymentRequest_MissingDriverID(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	paymentRepo := new(mockPaymentRequestRepository)
	uc := newUCWithPaymentRepo(repo, paymentRepo)

	got, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-1",
		AmountIDR:     5000,
		Method:        "QRIS",
	})

	require.Error(t, err)
	assert.Nil(t, got)
	var ae *apperror.AppError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, "VALIDATION", ae.Code)
	repo.AssertNotCalled(t, "GetByReservationID")
	paymentRepo.AssertNotCalled(t, "Create")
}

func TestCreatePaymentRequest_InvalidMethod(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	paymentRepo := new(mockPaymentRequestRepository)
	uc := newUCWithPaymentRepo(repo, paymentRepo)

	got, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-1",
		DriverID:      "driver-1",
		AmountIDR:     5000,
		Method:        "BANK_TRANSFER",
	})

	require.Error(t, err)
	assert.Nil(t, got)
	var ae *apperror.AppError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, "VALIDATION", ae.Code)
	repo.AssertNotCalled(t, "GetByReservationID")
	paymentRepo.AssertNotCalled(t, "Create")
}

func TestCreatePaymentRequest_CCRequiresToken(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	paymentRepo := new(mockPaymentRequestRepository)
	uc := newUCWithPaymentRepo(repo, paymentRepo)

	got, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-1",
		DriverID:      "driver-1",
		AmountIDR:     5000,
		Method:        "CC",
	})

	require.Error(t, err)
	assert.Nil(t, got)
	var ae *apperror.AppError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, "VALIDATION", ae.Code)
	repo.AssertNotCalled(t, "GetByReservationID")
	paymentRepo.AssertNotCalled(t, "Create")
}

func TestCreatePaymentRequest_QRISHappyPath(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	paymentRepo := new(mockPaymentRequestRepository)
	uc := newUCWithPaymentRepo(repo, paymentRepo)

	invoice := &model.Invoice{ID: "inv-1", ReservationID: "res-1", Status: model.InvoiceOpen}
	created := &model.PaymentRequest{
		ID:            "pr-1",
		ReservationID: "res-1",
		InvoiceID:     "inv-1",
		AmountIDR:     5000,
		Method:        model.PaymentMethodQRIS,
		Status:        model.PaymentRequestPending,
		PaymentRef:    "PAY-pr-1",
		QRISURL:       "https://qris.example.com/pay?ref=res-1&amount=5000",
		ExpiresAt:     time.Now().Add(15 * time.Minute),
	}

	repo.On("GetByReservationID", ctx, "res-1").Return(invoice, nil)
	paymentRepo.On("Create", ctx, mock.MatchedBy(func(req *model.PaymentRequest) bool {
		return req.ReservationID == "res-1" &&
			req.InvoiceID == "inv-1" &&
			req.AmountIDR == 5000 &&
			req.Method == model.PaymentMethodQRIS &&
			req.Status == model.PaymentRequestPending &&
			req.QRISURL != ""
	})).Return(created, nil)

	got, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-1",
		DriverID:      "driver-1",
		AmountIDR:     5000,
		Method:        "QRIS",
	})

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "pr-1", got.ID)
	assert.Equal(t, "QRIS", got.Method)
	assert.Equal(t, "PENDING", got.Status)
	assert.Equal(t, created.QRISURL, got.QRISURL)
	assert.Equal(t, "PAY-pr-1", got.PaymentRef)
	assert.NotZero(t, got.ExpiresAt)
	repo.AssertExpectations(t)
	paymentRepo.AssertExpectations(t)
}

func TestCreatePaymentRequest_CCHappyPath(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	paymentRepo := new(mockPaymentRequestRepository)
	uc := newUCWithPaymentRepo(repo, paymentRepo)

	invoice := &model.Invoice{ID: "inv-1", ReservationID: "res-1", Status: model.InvoiceOpen}
	created := &model.PaymentRequest{
		ID:            "pr-2",
		ReservationID: "res-1",
		InvoiceID:     "inv-1",
		AmountIDR:     5000,
		Method:        model.PaymentMethodCC,
		Status:        model.PaymentRequestPending,
		PaymentRef:    "PAY-pr-2",
		ExpiresAt:     time.Now().Add(15 * time.Minute),
	}

	repo.On("GetByReservationID", ctx, "res-1").Return(invoice, nil)
	paymentRepo.On("Create", ctx, mock.MatchedBy(func(req *model.PaymentRequest) bool {
		return req.ReservationID == "res-1" &&
			req.InvoiceID == "inv-1" &&
			req.AmountIDR == 5000 &&
			req.Method == model.PaymentMethodCC &&
			req.Status == model.PaymentRequestPending &&
			req.QRISURL == ""
	})).Return(created, nil)

	got, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-1",
		DriverID:      "driver-1",
		AmountIDR:     5000,
		Method:        "CC",
		CCToken:       "tok_123",
	})

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "pr-2", got.ID)
	assert.Equal(t, "CC", got.Method)
	assert.Equal(t, "PENDING", got.Status)
	assert.Empty(t, got.QRISURL)
	assert.Equal(t, "PAY-pr-2", got.PaymentRef)
	assert.NotZero(t, got.ExpiresAt)
	repo.AssertExpectations(t)
	paymentRepo.AssertExpectations(t)
}

func TestCreatePaymentRequest_DefaultsToQRIS(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	paymentRepo := new(mockPaymentRequestRepository)
	uc := newUCWithPaymentRepo(repo, paymentRepo)

	invoice := &model.Invoice{ID: "inv-1", ReservationID: "res-1", Status: model.InvoiceOpen}
	created := &model.PaymentRequest{
		ID:            "pr-3",
		ReservationID: "res-1",
		InvoiceID:     "inv-1",
		AmountIDR:     5000,
		Method:        model.PaymentMethodQRIS,
		Status:        model.PaymentRequestPending,
		PaymentRef:    "PAY-pr-3",
		QRISURL:       "https://qris.example.com/pay?ref=res-1&amount=5000",
		ExpiresAt:     time.Now().Add(15 * time.Minute),
	}

	repo.On("GetByReservationID", ctx, "res-1").Return(invoice, nil)
	paymentRepo.On("Create", ctx, mock.MatchedBy(func(req *model.PaymentRequest) bool {
		return req.Method == model.PaymentMethodQRIS && req.QRISURL != ""
	})).Return(created, nil)

	got, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-1",
		DriverID:      "driver-1",
		AmountIDR:     5000,
	})

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "QRIS", got.Method)
	repo.AssertExpectations(t)
	paymentRepo.AssertExpectations(t)
}

func TestCreatePaymentRequest_MissingPaymentRepo(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	uc := usecase.NewBillingUsecase(
		repo,
		pricing.NewDefaultEngine(pricing.DefaultConfig()),
		pricing.DefaultConfig(),
	)

	got, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-1",
		DriverID:      "driver-1",
		AmountIDR:     5000,
		Method:        "QRIS",
	})

	require.Error(t, err)
	assert.Nil(t, got)
	var ae *apperror.AppError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, "INTERNAL", ae.Code)
	repo.AssertNotCalled(t, "GetByReservationID")
}

func TestCreatePaymentRequest_PropagatesInvoiceLookupError(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	paymentRepo := new(mockPaymentRequestRepository)
	uc := newUCWithPaymentRepo(repo, paymentRepo)

	repo.On("GetByReservationID", ctx, "res-1").Return((*model.Invoice)(nil), apperror.ErrNotFound)

	got, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-1",
		DriverID:      "driver-1",
		AmountIDR:     5000,
		Method:        "QRIS",
	})

	require.Error(t, err)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	paymentRepo.AssertNotCalled(t, "Create")
	repo.AssertExpectations(t)
}

func TestCreatePaymentRequest_PropagatesCreateError(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	paymentRepo := new(mockPaymentRequestRepository)
	uc := newUCWithPaymentRepo(repo, paymentRepo)

	invoice := &model.Invoice{ID: "inv-1", ReservationID: "res-1", Status: model.InvoiceOpen}
	repo.On("GetByReservationID", ctx, "res-1").Return(invoice, nil)
	paymentRepo.On("Create", ctx, mock.Anything).Return((*model.PaymentRequest)(nil), apperror.ErrConflict)

	got, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-1",
		DriverID:      "driver-1",
		AmountIDR:     5000,
		Method:        "QRIS",
	})

	require.Error(t, err)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, apperror.ErrConflict)
	repo.AssertExpectations(t)
	paymentRepo.AssertExpectations(t)
}

func TestCreatePaymentRequest_InvalidAmount(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	paymentRepo := new(mockPaymentRequestRepository)
	uc := newUCWithPaymentRepo(repo, paymentRepo)

	got, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-1",
		DriverID:      "driver-1",
		AmountIDR:     0,
		Method:        "QRIS",
	})

	require.Error(t, err)
	assert.Nil(t, got)
	var ae *apperror.AppError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, "VALIDATION", ae.Code)
	repo.AssertNotCalled(t, "GetByReservationID")
	paymentRepo.AssertNotCalled(t, "Create")
}

func TestCreatePaymentRequest_WhitespaceMethodDefaultsToQRIS(t *testing.T) {
	ctx := context.Background()
	repo := new(mockrepo.MockInvoiceRepository)
	paymentRepo := new(mockPaymentRequestRepository)
	uc := newUCWithPaymentRepo(repo, paymentRepo)

	invoice := &model.Invoice{ID: "inv-1", ReservationID: "res-1", Status: model.InvoiceOpen}
	created := &model.PaymentRequest{
		ID:            "pr-4",
		ReservationID: "res-1",
		InvoiceID:     "inv-1",
		AmountIDR:     5000,
		Method:        model.PaymentMethodQRIS,
		Status:        model.PaymentRequestPending,
		PaymentRef:    "PAY-pr-4",
		QRISURL:       "https://qris.example.com/pay?ref=res-1&amount=5000",
		ExpiresAt:     time.Now().Add(15 * time.Minute),
	}

	repo.On("GetByReservationID", ctx, "res-1").Return(invoice, nil)
	paymentRepo.On("Create", ctx, mock.MatchedBy(func(req *model.PaymentRequest) bool {
		return req.Method == model.PaymentMethodQRIS
	})).Return(created, nil)

	got, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-1",
		DriverID:      "driver-1",
		AmountIDR:     5000,
		Method:        "   ",
	})

	require.NoError(t, err)
	assert.Equal(t, "QRIS", got.Method)
	repo.AssertExpectations(t)
	paymentRepo.AssertExpectations(t)
}

var _ repository.PaymentRequestRepository = (*mockPaymentRequestRepository)(nil)
