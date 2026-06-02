package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/farid/billing-service/internal/billing/model"
	apperror "github.com/farid/billing-service/pkg/error"
	"github.com/farid/billing-service/pkg/pricing"
)

// getOpenInvoice retrieves an invoice by reservation ID and returns it only if
// still OPEN. Returns (nil, nil) when the invoice doesn't exist (legitimate no-op)
// or is already closed (idempotent no-op).
func (u *billingUsecase) getOpenInvoice(ctx context.Context, reservationID string) (*model.Invoice, error) {
	inv, err := u.repo.GetByReservationID(ctx, reservationID)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if inv.Status != model.InvoiceOpen {
		return nil, nil
	}
	return inv, nil
}

// appendFee is the shared logic for appending a fee line to an open invoice
// and publishing the corresponding outbox event.
func (u *billingUsecase) appendFee(ctx context.Context, inv *model.Invoice, reservationID string, kind model.LineKind, amountIDR int64, reason string) error {
	payload, err := json.Marshal(map[string]any{
		"invoice_id":     inv.ID,
		"reservation_id": reservationID,
		"kind":           string(kind),
		"amount_idr":     amountIDR,
	})
	if err != nil {
		return fmt.Errorf("billing: marshal %s payload: %w", kind, err)
	}
	_, err = u.repo.AppendLine(ctx, inv.ID, model.LineItem{
		Kind: kind, AmountIDR: amountIDR,
		Metadata: map[string]any{"reason": reason},
	}, payload)
	return err
}

// ApplyCancelFee runs the pricing engine in cancel-only mode and appends the
// resulting line (if any) to the existing invoice for the given reservation.
// Idempotent: if the invoice is already CLOSED, do nothing.
func (u *billingUsecase) ApplyCancelFee(ctx context.Context, reservationID string, confirmedAt, cancelledAt time.Time) error {
	inv, err := u.getOpenInvoice(ctx, reservationID)
	if err != nil || inv == nil {
		return err
	}

	lines := u.engine.Apply(pricing.Session{
		ConfirmedAt: confirmedAt,
		CancelledAt: &cancelledAt,
		Timezone:    jakarta,
	})
	for _, l := range lines {
		if l.Kind != pricing.LineCancellation || l.AmountIDR == 0 {
			continue
		}
		if err := u.appendFee(ctx, inv, reservationID, model.LineCancellation, l.AmountIDR, "post-grace cancel"); err != nil {
			return err
		}
	}
	return nil
}

// ApplyNoShowFee appends a NOSHOW line. Same idempotency story as cancel.
func (u *billingUsecase) ApplyNoShowFee(ctx context.Context, reservationID string) error {
	inv, err := u.getOpenInvoice(ctx, reservationID)
	if err != nil || inv == nil {
		return err
	}

	return u.appendFee(ctx, inv, reservationID, model.LineNoShow, u.cfg.NoShowFeeIDR, "no-show")
}

// jakarta is UTC+7. time.FixedZone never fails, avoiding a panic at init time.
var jakarta = time.FixedZone("Asia/Jakarta", 7*60*60)
