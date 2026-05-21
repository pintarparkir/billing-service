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
}

type billingUsecase struct {
	repo   repository.InvoiceRepository
	engine *pricing.Engine
	cfg    pricing.Config
	users  grpcclient.UserClient
}

func NewBillingUsecase(repo repository.InvoiceRepository, engine *pricing.Engine, cfg pricing.Config) BillingUsecase {
	return &billingUsecase{repo: repo, engine: engine, cfg: cfg}
}

func (u *billingUsecase) WithUserClient(users grpcclient.UserClient) *billingUsecase {
	u.users = users
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
