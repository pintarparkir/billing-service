package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/farid/billing-service/internal/billing/model"
	"github.com/farid/billing-service/internal/billing/repository"
	mockrepo "github.com/farid/billing-service/internal/billing/repository/mock"
	grpcclient "github.com/farid/billing-service/pkg/grpcclient"
	"github.com/farid/billing-service/internal/billing/usecase"
	apperror "github.com/farid/billing-service/pkg/error"
	"github.com/farid/billing-service/pkg/pricing"
)

// TestCreatePaymentRequest_QRISWithPaymentService tests the full flow:
// billing calls payment-service gRPC → Midtrans SNAP → return RedirectURL
func TestCreatePaymentRequest_QRISWithPaymentService(t *testing.T) {
	ctx := context.Background()

	// Setup mocks
	repo := new(mockrepo.MockInvoiceRepository)
	paymentRepo := new(mockPaymentRequestRepository)
	paymentClient := new(mockPaymentClient)

	// Wire up usecase with all dependencies
	uc := usecase.NewBillingUsecase(repo, pricing.NewDefaultEngine(pricing.DefaultConfig()), pricing.DefaultConfig()).
		WithPaymentRequestRepository(paymentRepo).
		WithPaymentClient(paymentClient)

	// Arrange: Invoice exists
	invoice := &model.Invoice{ID: "inv-1", ReservationID: "res-1", Status: model.InvoiceOpen}
	repo.On("GetByReservationID", ctx, "res-1").Return(invoice, nil)

	// Arrange: Payment service returns SNAP redirect URL
	snapResult := &grpcclient.QrisIntentResult{
		PaymentID:   "pay-snap-123",
		SnapToken:   "SNAP-TOKEN-xyz",
		RedirectURL: "https://app.midtrans.com/snap/v1/transactions/pay-snap-123/pay",
		PgReference: "txn-snaphash-abc",
		ExpiresAt:   time.Now().Add(15 * time.Minute),
	}
	paymentClient.On("CreateQrisIntent", ctx, "inv-1", int64(5000)).Return(snapResult, nil)

	// Arrange: Payment request will be saved
	createdPR := &model.PaymentRequest{
		ID:            "pr-snap-1",
		ReservationID: "res-1",
		InvoiceID:     "inv-1",
		AmountIDR:     5000,
		Method:        model.PaymentMethodQRIS,
		Status:        model.PaymentRequestPending,
		PaymentRef:    "PAY-pr-snap-1",
		QRISURL:       snapResult.RedirectURL, // SNAP redirect_url is stored here
		ExpiresAt:     time.Now().Add(15 * time.Minute),
	}
	paymentRepo.On("Create", ctx, mock.MatchedBy(func(req *model.PaymentRequest) bool {
		return req.Method == model.PaymentMethodQRIS && req.QRISURL == snapResult.RedirectURL
	})).Return(createdPR, nil)

	// Act
	result, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-1",
		DriverID:      "driver-1",
		AmountIDR:     5000,
		Method:        "QRIS",
	})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "pr-snap-1", result.ID)
	assert.Equal(t, "QRIS", result.Method)
	assert.Equal(t, "PENDING", result.Status)
	assert.Equal(t, snapResult.RedirectURL, result.QRISURL)
	assert.NotEmpty(t, result.PaymentRef)
	assert.NotZero(t, result.ExpiresAt)

	// Verify calls
	repo.AssertExpectations(t)
	paymentRepo.AssertExpectations(t)
	paymentClient.AssertExpectations(t)
}

// TestCreatePaymentRequest_CCFlow tests auto-debit path where no payment-service call is needed
func TestCreatePaymentRequest_CCAutoDebitFlow(t *testing.T) {
	ctx := context.Background()

	repo := new(mockrepo.MockInvoiceRepository)
	paymentRepo := new(mockPaymentRequestRepository)
	paymentClient := new(mockPaymentClient) // Not used for CC but wired

	uc := usecase.NewBillingUsecase(repo, pricing.NewDefaultEngine(pricing.DefaultConfig()), pricing.DefaultConfig()).
		WithPaymentRequestRepository(paymentRepo).
		WithPaymentClient(paymentClient)

	invoice := &model.Invoice{ID: "inv-2", ReservationID: "res-2", Status: model.InvoiceOpen}
	repo.On("GetByReservationID", ctx, "res-2").Return(invoice, nil)

	// For CC, QRISURL should remain empty
	createdPR := &model.PaymentRequest{
		ID:            "pr-cc-1",
		ReservationID: "res-2",
		InvoiceID:     "inv-2",
		AmountIDR:     5000,
		Method:        model.PaymentMethodCC,
		Status:        model.PaymentRequestPending,
		PaymentRef:    "PAY-pr-cc-1",
		ExpiresAt:     time.Now().Add(15 * time.Minute),
	}
	paymentRepo.On("Create", ctx, mock.MatchedBy(func(req *model.PaymentRequest) bool {
		return req.Method == model.PaymentMethodCC && req.QRISURL == ""
	})).Return(createdPR, nil)

	result, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-2",
		DriverID:      "driver-1",
		AmountIDR:     5000,
		Method:        "CC",
		CCToken:       "tok_card_123",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "CC", result.Method)
	assert.Empty(t, result.QRISURL) // No QRIS for CC
	assert.Equal(t, "PAY-pr-cc-1", result.PaymentRef)

	repo.AssertExpectations(t)
	paymentRepo.AssertExpectations(t)
	// Payment client should NOT be called for CC method
	paymentClient.AssertNotCalled(t, "CreateQrisIntent")
}

// TestCreatePaymentRequest_PaymentServiceError tests graceful failure when payment-service unavailable
func TestCreatePaymentRequest_PaymentServiceUnreachable(t *testing.T) {
	ctx := context.Background()

	repo := new(mockrepo.MockInvoiceRepository)
	paymentRepo := new(mockPaymentRequestRepository)
	paymentClient := new(mockPaymentClient)

	uc := usecase.NewBillingUsecase(repo, pricing.NewDefaultEngine(pricing.DefaultConfig()), pricing.DefaultConfig()).
		WithPaymentRequestRepository(paymentRepo).
		WithPaymentClient(paymentClient)

	invoice := &model.Invoice{ID: "inv-3", ReservationID: "res-3", Status: model.InvoiceOpen}
	repo.On("GetByReservationID", ctx, "res-3").Return(invoice, nil)

	// Payment service fails
	paymentClient.On("CreateQrisIntent", ctx, "inv-3", int64(5000)).Return(nil, apperror.ErrUpstreamDown)

	// Fallback: create PR without QRIS URL
	createdPR := &model.PaymentRequest{
		ID:            "pr-fallback-1",
		ReservationID: "res-3",
		InvoiceID:     "inv-3",
		AmountIDR:     5000,
		Method:        model.PaymentMethodQRIS,
		Status:        model.PaymentRequestPending,
		PaymentRef:    "PAY-pr-fallback-1",
		QRISURL:       "", // Empty because payment service failed
		ExpiresAt:     time.Now().Add(15 * time.Minute),
	}
	paymentRepo.On("Create", ctx, mock.Anything).Return(createdPR, nil)

	result, err := uc.CreatePaymentRequest(ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-3",
		DriverID:      "driver-1",
		AmountIDR:     5000,
		Method:        "QRIS",
	})

	// Should succeed but QRIS URL will be empty (requires manual handling later)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "QRIS", result.Method)
	assert.Empty(t, result.QRISURL) // Fallback behavior - QRIS URL missing

	repo.AssertExpectations(t)
	paymentRepo.AssertExpectations(t)
	paymentClient.AssertExpectations(t)
}

var _ repository.PaymentRequestRepository = (*mockPaymentRequestRepository)(nil)
var _ grpcclient.PaymentClient = (*mockPaymentClient)(nil)
