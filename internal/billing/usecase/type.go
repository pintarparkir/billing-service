// Package usecase orchestrates billing-domain business logic.
package usecase

import (
	"context"
	"time"

	"github.com/farid/billing-service/internal/billing/model"
	"github.com/farid/billing-service/internal/billing/repository"
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
}

type billingUsecase struct {
	repo   repository.InvoiceRepository
	engine *pricing.Engine
	cfg    pricing.Config
}

func NewBillingUsecase(repo repository.InvoiceRepository, engine *pricing.Engine, cfg pricing.Config) BillingUsecase {
	return &billingUsecase{repo: repo, engine: engine, cfg: cfg}
}
