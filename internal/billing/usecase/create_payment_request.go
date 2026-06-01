package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/farid/billing-service/internal/billing/model"
	apperror "github.com/farid/billing-service/pkg/error"
)

type CreatePaymentRequestInput struct {
	ReservationID string
	DriverID      string
	AmountIDR     int64
	Method        string
	CCToken       string
}

type CreatePaymentRequestOutput struct {
	ID        string
	Method    string
	Status    string
	QRISURL   string
	PaymentRef string
	ExpiresAt int64
}

// CreatePaymentRequest creates a payment request for a reservation.
// It validates the input, checks the invoice exists, and creates a payment request.
func (u *billingUsecase) CreatePaymentRequest(ctx context.Context, input CreatePaymentRequestInput) (*CreatePaymentRequestOutput, error) {
	if strings.TrimSpace(input.ReservationID) == "" {
		return nil, &apperror.AppError{Code: "VALIDATION", Message: "reservation_id required"}
	}
	if strings.TrimSpace(input.DriverID) == "" {
		return nil, &apperror.AppError{Code: "VALIDATION", Message: "driver_id required"}
	}
	if input.AmountIDR <= 0 {
		return nil, &apperror.AppError{Code: "VALIDATION", Message: "amount_idr must be positive"}
	}

	method := strings.TrimSpace(input.Method)
	if method == "" {
		method = "QRIS"
	}

	// Validate method
	if method != "QRIS" && method != "CC" {
		return nil, &apperror.AppError{Code: "VALIDATION", Message: "method must be QRIS or CC"}
	}

	// If CC method, require token
	if method == "CC" && strings.TrimSpace(input.CCToken) == "" {
		return nil, &apperror.AppError{Code: "VALIDATION", Message: "cc_token required for CC method"}
	}

	// Ensure payment repo is wired
	if u.paymentRepo == nil {
		return nil, &apperror.AppError{Code: "INTERNAL", Message: "payment request repository not available"}
	}

	// Get invoice to verify it exists
	invoice, err := u.repo.GetByReservationID(ctx, input.ReservationID)
	if err != nil {
		return nil, err
	}

	// Create payment request
	paymentMethod := model.PaymentMethod(method)
	expiresAt := time.Now().Add(15 * time.Minute)

	paymentReq := &model.PaymentRequest{
		ReservationID: input.ReservationID,
		InvoiceID:     invoice.ID,
		AmountIDR:     input.AmountIDR,
		Method:        paymentMethod,
		Status:        model.PaymentRequestPending,
		ExpiresAt:     expiresAt,
	}

	// Generate QRIS URL if method is QRIS
	if paymentMethod == model.PaymentMethodQRIS {
		paymentReq.QRISURL = u.generateQRISURL(input.ReservationID, input.AmountIDR)
	}

	created, err := u.paymentRepo.Create(ctx, paymentReq)
	if err != nil {
		return nil, err
	}

	return &CreatePaymentRequestOutput{
		ID:        created.ID,
		Method:    string(created.Method),
		Status:    string(created.Status),
		QRISURL:   created.QRISURL,
		PaymentRef: created.PaymentRef,
		ExpiresAt: created.ExpiresAt.Unix(),
	}, nil
}

// generateQRISURL generates a mock QRIS URL for the payment request.
// In production, this would call the actual QRIS provider API.
func (u *billingUsecase) generateQRISURL(reservationID string, amountIDR int64) string {
	return fmt.Sprintf("https://qris.example.com/pay?ref=%s&amount=%d", reservationID, amountIDR)
}
