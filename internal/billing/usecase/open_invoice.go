package usecase

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/farid/billing-service/internal/billing/model"
	apperror "github.com/farid/billing-service/pkg/error"
)

func (u *billingUsecase) OpenInvoice(ctx context.Context, reservationID, driverID, idem string) (*model.Invoice, error) {
	if strings.TrimSpace(reservationID) == "" {
		return nil, &apperror.AppError{Code: "VALIDATION", Message: "reservation_id required"}
	}
	if strings.TrimSpace(idem) == "" {
		return nil, &apperror.AppError{Code: "VALIDATION", Message: "idempotency_key required"}
	}

	payload, _ := json.Marshal(map[string]any{
		"reservation_id":  reservationID,
		"driver_id":       driverID,
		"booking_fee_idr": u.cfg.BookingFeeIDR,
		"idempotency_key": idem,
	})
	return u.repo.Open(ctx, reservationID, driverID, idem, u.cfg.BookingFeeIDR, payload)
}
