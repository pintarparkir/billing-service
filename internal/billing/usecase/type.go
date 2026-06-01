// Package usecase orchestrates billing-domain business logic.
package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/farid/billing-service/internal/billing/model"
	"github.com/farid/billing-service/internal/billing/repository"
	"github.com/farid/billing-service/pkg/grpcclient"
	"github.com/farid/billing-service/pkg/pricing"
)

type BillingUsecase interface {
	OpenInvoice(ctx context.Context, reservationID, driverID, idempotencyKey string) (*model.Invoice, error)
	CloseInvoice(ctx context.Context, invoiceID string, session pricing.Session) (*model.Invoice, error)
	GetInvoice(ctx context.Context, id string) (*model.Invoice, error)
	GetInvoiceByReservation(ctx context.Context, reservationID string) (*model.Invoice, error)

	// ApplyCancelFee is invoked from the RabbitMQ consumer on
	// reservation.cancelled.v1. cancelDelta = duration since confirm; engine
	// decides 0 (grace) or CancelFeeIDR.
	ApplyCancelFee(ctx context.Context, reservationID string, confirmedAt, cancelledAt time.Time) error

	// ApplyNoShowFee is invoked from reservation.expired.v1.
	ApplyNoShowFee(ctx context.Context, reservationID string) error

	// CreatePaymentRequest creates a booking-fee payment request for a reservation.
	// If method=QRIS, returns a QRIS URL. If method=CC, triggers auto-debit via CCToken.
	CreatePaymentRequest(ctx context.Context, input CreatePaymentRequestInput) (*CreatePaymentRequestOutput, error)

	// HandlePaymentNotification processes webhook callbacks from payment gateway.
	// Verifies signature, updates PR status, and emits success/failed event via outbox.
	HandlePaymentNotification(ctx context.Context, payload []byte) error

	WithPaymentRequestRepository(paymentRepo repository.PaymentRequestRepository) BillingUsecase
}

type billingUsecase struct {
	repo        repository.InvoiceRepository
	paymentRepo repository.PaymentRequestRepository
	engine      *pricing.Engine
	cfg         pricing.Config
	users       grpcclient.UserClient
	payment     grpcclient.PaymentClient
}

func NewBillingUsecase(repo repository.InvoiceRepository, engine *pricing.Engine, cfg pricing.Config) BillingUsecase {
	return &billingUsecase{repo: repo, engine: engine, cfg: cfg}
}

func (u *billingUsecase) WithUserClient(users grpcclient.UserClient) *billingUsecase {
	u.users = users
	return u
}

// WithPaymentRequestRepository wires in the payment request repository.
// Call this in main after NewBillingUsecase when the payment_requests table is ready.
func (u *billingUsecase) WithPaymentRequestRepository(paymentRepo repository.PaymentRequestRepository) BillingUsecase {
	u.paymentRepo = paymentRepo
	return u
}

// WithPaymentClient wires in the payment-service gRPC client for QRIS intent creation.
func (u *billingUsecase) WithPaymentClient(client grpcclient.PaymentClient) *billingUsecase {
	u.payment = client
	return u
}

func (u *billingUsecase) lookupMSISDN(ctx context.Context, driverID string) string {
	if u.users == nil || driverID == "" {
		return ""
	}
	msisdn, err := u.users.GetMSISDN(ctx, driverID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(msisdn)
}
