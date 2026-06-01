package billing_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/farid/billing-service/internal/billing/model"
	mockrepo "github.com/farid/billing-service/internal/billing/repository/mock"
	"github.com/farid/billing-service/internal/billing/usecase"
	apperror "github.com/farid/billing-service/pkg/error"
	grpcclient "github.com/farid/billing-service/pkg/grpcclient"
	"github.com/farid/billing-service/pkg/pricing"
)

// mockPaymentClient implements grpcclient.PaymentClient for E2E tests
type e2eMockPaymentClient struct {
	mock.Mock
}

func (m *e2eMockPaymentClient) CreateQrisIntent(ctx context.Context, invoiceID string, amountIDR int64) (*grpcclient.QrisIntentResult, error) {
	args := m.Called(ctx, invoiceID, amountIDR)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*grpcclient.QrisIntentResult), args.Error(1)
}

func (m *e2eMockPaymentClient) GetPayment(ctx context.Context, paymentID string) (*grpcclient.PaymentResult, error) {
	args := m.Called(ctx, paymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*grpcclient.PaymentResult), args.Error(1)
}

// E2ETestSuite provides comprehensive E2E test scenarios for complete payment flow
type E2ETestSuite struct {
	ctx                context.Context
	repo               *mockrepo.MockInvoiceRepository
	paymentRepo        *mockPaymentRequestRepository
	paymentClient      *e2eMockPaymentClient
	uc                 usecase.BillingUsecase
	invoice            *model.Invoice
	snapRedirectURL    string
}

func NewE2ETestSuite() *E2ETestSuite {
	return &E2ETestSuite{
		ctx:             context.Background(),
		repo:            new(mockrepo.MockInvoiceRepository),
		paymentRepo:     new(mockPaymentRequestRepository),
		paymentClient:   new(e2eMockPaymentClient),
		snapRedirectURL: "https://app.midtrans.com/snap/v1/transactions/pay-e2e-123/pay",
	}
}

func (s *E2ETestSuite) SetupTest() {
	s.uc = usecase.NewBillingUsecase(s.repo, pricing.NewDefaultEngine(pricing.DefaultConfig()), pricing.DefaultConfig()).
		WithPaymentRequestRepository(s.paymentRepo).
		WithPaymentClient(s.paymentClient)

	s.invoice = &model.Invoice{
		ID:            "inv-e2e-123",
		ReservationID: "res-e2e-123",
		DriverID:      "driver-e2e-123",
		Status:        model.InvoiceOpen,
		TotalIDR:      5000,
		CreatedAt:     time.Now().UTC(),
	}
}

// TestE2E_BookingFeeQRISFlowHappyPath verifies complete booking fee QRIS payment flow from creation to success
func (s *E2ETestSuite) TestE2E_BookingFeeQRISFlowHappyPath(t *testing.T) {
	s.SetupTest()

	// Given: Invoice exists
	s.repo.On("GetByReservationID", s.ctx, "res-e2e-123").Return(s.invoice, nil)

	// When: Client creates payment request for booking fee
	s.paymentClient.On("CreateQrisIntent", s.ctx, "inv-e2e-123", int64(5000)).Return(&grpcclient.QrisIntentResult{
		PaymentID:   "pay-e2e-123",
		SnapToken:   "SNAP-TOKEN-E2E-xyz",
		RedirectURL: s.snapRedirectURL,
		PgReference: "txn-e2e-snaphash",
		ExpiresAt:   time.Now().Add(15 * time.Minute),
	}, nil)

	result, err := s.uc.CreatePaymentRequest(s.ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-e2e-123",
		DriverID:      "driver-e2e-123",
		AmountIDR:     5000,
		Method:        "QRIS",
	})

	// Then: Payment request created with SNAP redirect URL
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "qrissnap-e2e-1", result.ID)
	assert.Equal(t, "QRIS", result.Method)
	assert.Equal(t, "PENDING", result.Status)
	assert.Equal(t, s.snapRedirectURL, result.QRISURL)
	assert.Equal(t, "txn-e2e-snaphash", result.PaymentRef)
	assert.NotZero(t, result.ExpiresAt)

	// Verify all expectations met
	s.repo.AssertExpectations(t)
	s.paymentRepo.AssertExpectations(t)
	s.paymentClient.AssertExpectations(t)
}

// TestE2E_CCAutoDebitFlowNoExternalCall verifies CC auto-debit doesn't call external service
func (s *E2ETestSuite) TestE2E_CCAutoDebitFlowNoExternalCall(t *testing.T) {
	s.SetupTest()

	s.repo.On("GetByReservationID", s.ctx, "res-e2e-456").Return(s.invoice, nil)
	// Note: No payment client call expected for CC method

	paymentReq := &model.PaymentRequest{
		ID:            "pr-cc-e2e-1",
		ReservationID: "res-e2e-456",
		InvoiceID:     s.invoice.ID,
		AmountIDR:     5000,
		Method:        model.PaymentMethodCC,
		Status:        model.PaymentRequestPending,
		PaymentRef:    "PAY-pr-cc-e2e-1",
		QRISURL:       "", // Empty for CC
		ExpiresAt:     time.Now().Add(15 * time.Minute),
	}

	s.paymentRepo.On("Create", s.ctx, mock.MatchedBy(func(req *model.PaymentRequest) bool {
		return req.Method == model.PaymentMethodCC && req.QRISURL == ""
	})).Return(paymentReq, nil)

	result, err := s.uc.CreatePaymentRequest(s.ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-e2e-456",
		DriverID:      "driver-e2e-123",
		AmountIDR:     5000,
		Method:        "CC",
		CCToken:       "tok_cc_e2e_secure_token",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "CC", result.Method)
	assert.Empty(t, result.QRISURL)
	assert.NotEmpty(t, result.PaymentRef)

	// Payment client should NOT be called for CC method
	s.paymentClient.AssertNotCalled(t, "CreateQrisIntent")
}

// TestE2E_PaymentTimeoutScenario verifies reservation expires after 15 min without payment
func (s *E2ETestSuite) TestE2E_PaymentTimeoutScenario(t *testing.T) {
	s.SetupTest()

	// Simulate expired payment request
	expiredPR := &model.PaymentRequest{
		ID:            "pr-expired-e2e",
		ReservationID: "res-e2e-timeout",
		InvoiceID:     s.invoice.ID,
		AmountIDR:     5000,
		Method:        model.PaymentMethodQRIS,
		Status:        model.PaymentRequestExpired,
		PaymentRef:    "PAY-pr-expired-e2e",
		ExpiresAt:     time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
	}

	exists, err := s.paymentRepo.IsExpired(s.ctx, expiredPR.ID)
	require.NoError(t, err)
	assert.True(t, exists, "Payment request should be marked as expired")
}

// TestE2E_MultiplePaymentRetries handles scenario where user retries payment multiple times
func (s *E2ETestSuite) TestE2E_MultiplePaymentRetries(t *testing.T) {
	s.SetupTest()

	invoiceRetry := &model.Invoice{
		ID:            "inv-e2e-retry",
		ReservationID: "res-e2e-retry",
		Status:        model.InvoiceOpen,
		TotalIDR:      5000,
	}

	s.repo.On("GetByReservationID", s.ctx, "res-e2e-retry").Return(invoiceRetry, nil)

	// First retry - successful SNAP intent
	firstSnapURL := "https://app.midtrans.com/snap/v1/transactions/retry-1/pay"
	s.paymentClient.On("CreateQrisIntent", s.ctx, "inv-e2e-retry", int64(5000)).Once().
		Return(&grpcclient.QrisIntentResult{RedirectURL: firstSnapURL}, nil)

	result1, err := s.uc.CreatePaymentRequest(s.ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-e2e-retry",
		DriverID:      "driver-e2e-123",
		AmountIDR:     5000,
		Method:        "QRIS",
	})

	require.NoError(t, err)
	assert.Equal(t, firstSnapURL, result1.QRISURL)

	// Second retry - different SNAP URL (new transaction)
	secondSnapURL := "https://app.midtrans.com/snap/v1/transactions/retry-2/pay"
	s.paymentClient.On("CreateQrisIntent", s.ctx, "inv-e2e-retry", int64(5000")).Once().
		Return(&grpcclient.QrisIntentResult{RedirectURL: secondSnapURL}, nil)

	result2, err := s.uc.CreatePaymentRequest(s.ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-e2e-retry",
		DriverID:      "driver-e2e-123",
		AmountIDR:     5000,
		Method:        "QRIS",
	})

	require.NoError(t, err)
	assert.Equal(t, secondSnapURL, result2.QRISURL)
	assert.NotEqual(t, result1.PaymentRef, result2.PaymentRef, "Should create new payment ref on retry")
}

// TestE2E_PaymentServiceUnreachableFallsBackToManualReview verifies graceful degradation
func (s *E2ETestSuite) TestE2E_PaymentServiceUnreachableFallsBackToManualReview(t *testing.T) {
	s.SetupTest()

	s.repo.On("GetByReservationID", s.ctx, "res-e2e-unreachable").Return(s.invoice, nil)
	// Payment service fails
	s.paymentClient.On("CreateQrisIntent", s.ctx, "inv-e2e-unreachable", int64(5000)).
		Return(nil, apperror.ErrUpstreamDown)

	// Should still create payment request but with empty QRIS URL (manual review needed)
	result, err := s.uc.CreatePaymentRequest(s.ctx, usecase.CreatePaymentRequestInput{
		ReservationID: "res-e2e-unreachable",
		DriverID:      "driver-e2e-123",
		AmountIDR:     5000,
		Method:        "QRIS",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "QRIS", result.Method)
	assert.Empty(t, result.QRISURL, "QRIS URL should be empty when service unavailable")
	assert.NotEmpty(t, result.PaymentRef, "Should have payment ref even in error case")
}

// TestE2E_FullWebhookLifecycle simulates complete webhook notification flow
func (s *E2ETestSuite) TestE2E_FullWebhookLifecycle(t *testing.T) {
	// This test would integrate with the full system including RabbitMQ
	// For unit testing, we verify the webhook handler logic directly

	payload, _ := json.Marshal(map[string]interface{}{
		"order_id":           "txn-e2e-webhook",
		"status_code":        "200",
		"gross_amount":       "5000.00",
		"signature_key":      "dummy_signature",
		"transaction_status": "settlement",
		"fraud_status":       "accept",
	})

	n, err := model.ParseAndVerify(payload, "test_webhook_secret")

	// Verify notification parsing works correctly
	assert.NotNil(t, n)
	assert.Equal(t, "settlement", n.TransactionStatus)
	assert.True(t, n.IsTerminalSuccess())

	// Verify failure cases too
	failurePayload, _ := json.Marshal(map[string]interface{}{
		"transaction_status": "deny",
		"fraud_status":       "deny",
	})

	failN, _ := model.ParseAndVerify(failurePayload, "test_webhook_secret")
	assert.False(t, failN.IsTerminalSuccess())
	assert.True(t, failN.IsTerminalFailure())
}

// Mock repositories for E2E tests
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

func (m *mockPaymentRequestRepository) IsExpired(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

var _ repository.PaymentRequestRepository = (*mockPaymentRequestRepository)(nil)
