// Package consumer wires RabbitMQ deliveries into the billing usecase.
//
// Subscribed routing keys:
//   reservation.cancelled.v1   → ApplyCancelFee
//   reservation.expired.v1     → ApplyNoShowFee
//   reservation.checked_out.v1 → CloseInvoice (full pricing)
package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/farid/billing-service/internal/billing/model"
	"github.com/farid/billing-service/internal/billing/usecase"
	"github.com/farid/billing-service/pkg/logger"
	"github.com/farid/billing-service/pkg/pricing"
)

// Reservation event payloads (subset we care about). The producer
// (reservation-service) emits these as JSON via outbox.
type cancelledEvt struct {
	ReservationID string    `json:"reservation_id"`
	Reason        string    `json:"reason"`
	ConfirmedAt   time.Time `json:"confirmed_at,omitempty"`
	CancelledAt   time.Time `json:"cancelled_at,omitempty"`
}

type expiredEvt struct {
	ReservationID string `json:"reservation_id"`
}

type checkedOutEvt struct {
	ReservationID string    `json:"reservation_id"`
	ConfirmedAt   time.Time `json:"confirmed_at,omitempty"`
	CheckedInAt   time.Time `json:"checked_in_at,omitempty"`
	CheckedOutAt  time.Time `json:"checked_out_at,omitempty"`
}

type Reservation struct {
	uc usecase.BillingUsecase
}

func NewReservation(uc usecase.BillingUsecase) *Reservation { return &Reservation{uc: uc} }

// Handle dispatches one delivery to the right usecase method.
// Returns non-nil to NACK + requeue; nil to ACK.
func (c *Reservation) Handle(ctx context.Context, routingKey string, body []byte) error {
	switch routingKey {
	case model.EvtReservationCancelled:
		var ev cancelledEvt
		if err := json.Unmarshal(body, &ev); err != nil {
			logger.Error(ctx, "consumer: bad payload", map[string]interface{}{
				"routing_key": routingKey, logger.ErrorKey: err.Error(),
			})
			return nil // bad payload → drop, don't requeue forever
		}
		// Fall back to "now" if producer didn't include the timestamps.
		if ev.CancelledAt.IsZero() {
			ev.CancelledAt = time.Now()
		}
		return c.uc.ApplyCancelFee(ctx, ev.ReservationID, ev.ConfirmedAt, ev.CancelledAt)

	case model.EvtReservationExpired:
		var ev expiredEvt
		if err := json.Unmarshal(body, &ev); err != nil {
			return nil
		}
		return c.uc.ApplyNoShowFee(ctx, ev.ReservationID)

	case model.EvtReservationCheckedOut:
		var ev checkedOutEvt
		if err := json.Unmarshal(body, &ev); err != nil {
			return nil
		}
		// Look up the invoice for this reservation, then close.
		inv, err := c.uc.GetInvoiceByReservation(ctx, ev.ReservationID)
		if err != nil {
			return err
		}
		if inv == nil {
			return nil // no matching invoice → drop
		}
		_, err = c.uc.CloseInvoice(ctx, inv.ID, pricing.Session{
			ConfirmedAt:  ev.ConfirmedAt,
			CheckedInAt:  ev.CheckedInAt,
			CheckedOutAt: ev.CheckedOutAt,
			VehicleType:  pricing.VehicleCar, // unknown from event; assume CAR for MVP tariffs
		})
		return err
	}
	return nil
}
