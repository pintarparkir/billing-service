// Package grpc adapts the CreatePaymentRequest gRPC method to the usecase layer.
package grpc

import (
	"context"

	billingv1 "github.com/farid/billing-service/api/proto/billing/v1"
	"github.com/farid/billing-service/internal/billing/usecase"
)

// CreatePaymentRequest creates a booking-fee payment request for a reservation.
// Supports QRIS payment (returns QRIS URL) or CC auto-debit (uses default CC token).
func (s *Server) CreatePaymentRequest(ctx context.Context, req *billingv1.CreatePaymentReq) (*billingv1.PaymentRequest, error) {
	input := usecase.CreatePaymentRequestInput{
		ReservationID: req.GetReservationId(),
		DriverID:      req.GetDriverId(),
		AmountIDR:     req.GetAmountIdr(),
		Method:        req.GetMethod(),
		CCToken:       req.GetCcToken(),
	}

	output, err := s.uc.CreatePaymentRequest(ctx, input)
	if err != nil {
		return nil, err
	}

	return &billingv1.PaymentRequest{
		Id:         output.ID,
		Method:     output.Method,
		Status:     output.Status,
		QrisUrl:    output.QRISURL,
		PaymentRef: output.PaymentRef,
		ExpiresAt:  output.ExpiresAt,
	}, nil
}
