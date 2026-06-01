package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/farid/billing-service/internal/billing/model"
	apperror "github.com/farid/billing-service/pkg/error"
)

// PaymentNotification represents webhook payload from payment gateway (Midtrans-like).
type PaymentNotification struct {
	OrderID           string `json:"order_id"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status,omitempty"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key,omitempty"`
}

func (u *billingUsecase) HandlePaymentNotification(ctx context.Context, payload []byte) error {
	if u.paymentRepo == nil {
		return &apperror.AppError{Code: "INTERNAL", Message: "payment request repository not available"}
	}

	var notif PaymentNotification
	if err := json.Unmarshal(payload, &notif); err != nil {
		return &apperror.AppError{Code: "VALIDATION", Message: "invalid notification payload"}
	}

	if notif.OrderID == "" {
		return &apperror.AppError{Code: "VALIDATION", Message: "order_id required"}
	}

	// Lookup payment request by order_id (which maps to payment_ref)
	pr, err := u.paymentRepo.GetByPaymentRef(ctx, notif.OrderID)
	if err != nil {
		return err
	}

	// Determine new status based on transaction_status
	var newStatus model.PaymentRequestStatus
	var eventType string

	switch notif.TransactionStatus {
	case "capture", "settlement":
		if notif.FraudStatus == "accept" || notif.FraudStatus == "" {
			newStatus = model.PaymentRequestSuccess
			eventType = model.EvtPaymentSuccess
		} else {
			newStatus = model.PaymentRequestFailed
			eventType = model.EvtPaymentFailed
		}
	case "pending":
		return nil // no state change needed
	case "deny", "cancel", "expire", "failure":
		newStatus = model.PaymentRequestFailed
		eventType = model.EvtPaymentFailed
	default:
		return nil // unknown status, ignore
	}

	// Build event payload
	eventPayload, err := json.Marshal(map[string]any{
		"reservation_id": pr.ReservationID,
		"invoice_id":     pr.InvoiceID,
		"payment_ref":    pr.PaymentRef,
		"amount_idr":     pr.AmountIDR,
		"method":         string(pr.Method),
		"status":         string(newStatus),
	})
	if err != nil {
		return fmt.Errorf("billing: marshal payment event: %w", err)
	}

	// Update status and emit event atomically
	_, err = u.paymentRepo.UpdateStatusByPaymentRef(ctx, notif.OrderID, newStatus, eventType, eventPayload)
	return err
}
