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

// ApplyCancelFee runs the pricing engine in cancel-only mode and appends the
// resulting line (if any) to the existing invoice for the given reservation.
// Idempotent: if the invoice is already CLOSED, do nothing.
func (u *billingUsecase) ApplyCancelFee(ctx context.Context, reservationID string, confirmedAt, cancelledAt time.Time) error {
	inv, err := u.repo.GetByReservationID(ctx, reservationID)
	if err != nil {
		// Invoice may not exist if reservation was cancelled before confirm.
		// That's a legitimate no-op — log and return nil.
		if errors.Is(err, apperror.ErrNotFound) {
			return nil
		}
		return err
	}
	if inv.Status != model.InvoiceOpen {
		return nil // already closed; idempotent no-op
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
		payload, merr := json.Marshal(map[string]any{
			"invoice_id":     inv.ID,
			"reservation_id": reservationID,
			"kind":           "CANCELLATION",
			"amount_idr":     l.AmountIDR,
		})
		if merr != nil {
			return fmt.Errorf("billing: marshal cancel payload: %w", merr)
		}
		if _, err := u.repo.AppendLine(ctx, inv.ID, model.LineItem{
			Kind: model.LineCancellation, AmountIDR: l.AmountIDR,
			Metadata: map[string]any{"reason": "post-grace cancel"},
		}, payload); err != nil {
			return err
		}
	}
	return nil
}

// ApplyNoShowFee appends a NOSHOW line. Same idempotency story as cancel.
func (u *billingUsecase) ApplyNoShowFee(ctx context.Context, reservationID string) error {
	inv, err := u.repo.GetByReservationID(ctx, reservationID)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return nil
		}
		return err
	}
	if inv.Status != model.InvoiceOpen {
		return nil
	}

	payload, err := json.Marshal(map[string]any{
		"invoice_id":     inv.ID,
		"reservation_id": reservationID,
		"kind":           "NOSHOW",
		"amount_idr":     u.cfg.NoShowFeeIDR,
	})
	if err != nil {
		return fmt.Errorf("billing: marshal noshow payload: %w", err)
	}
	_, err = u.repo.AppendLine(ctx, inv.ID, model.LineItem{
		Kind: model.LineNoShow, AmountIDR: u.cfg.NoShowFeeIDR,
		Metadata: map[string]any{"reason": "no-show"},
	}, payload)
	return err
}

// jakarta is UTC+7. time.FixedZone never fails, avoiding a panic at init time.
var jakarta = time.FixedZone("Asia/Jakarta", 7*60*60)
