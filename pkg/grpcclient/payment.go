// Package grpcclient holds payment-service gRPC client for billing-service.
package grpcclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	paymentv1 "github.com/farid/billing-service/api/proto/payment/v1"
)

// PaymentClient interface for payment-service gRPC calls.
type PaymentClient interface {
	CreateQrisIntent(ctx context.Context, invoiceID string, amountIDR int64) (*QrisIntentResult, error)
	GetPayment(ctx context.Context, paymentID string) (*PaymentResult, error)
}

// QrisIntentResult represents the response from CreateQrisIntent.
type QrisIntentResult struct {
	PaymentID   string
	SnapToken   string
	RedirectURL string
	PgReference string
	ExpiresAt   time.Time
}

// PaymentResult represents the response from GetPayment.
type PaymentResult struct {
	ID          string
	InvoiceID   string
	Method      string
	Status      string
	PgReference string
	AmountIDR   int64
	CreatedAt   time.Time
	PaidAt      *time.Time
}

type paymentClient struct {
	conn   *grpc.ClientConn
	client paymentv1.PaymentServiceClient
}

// NewPaymentClient creates a new gRPC client for payment-service.
func NewPaymentClient(addr string, timeout time.Duration) PaymentClient {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024)),
	)
	if err != nil {
		panic(fmt.Errorf("payment grpc dial %s: %w", addr, err))
	}
	return &paymentClient{
		conn:   conn,
		client: paymentv1.NewPaymentServiceClient(conn),
	}
}

// CreateQrisIntent calls payment-service to create a QRIS intent via Midtrans SNAP.
func (c *paymentClient) CreateQrisIntent(ctx context.Context, invoiceID string, amountIDR int64) (*QrisIntentResult, error) {
	req := &paymentv1.CreateQrisIntentRequest{
		InvoiceId: invoiceID,
		AmountIdr: amountIDR,
	}

	resp, err := c.client.CreateQrisIntent(ctx, req)
	if err != nil {
		return nil, err
	}

	var expiresAt time.Time
	if resp.ExpiresAt != nil {
		expiresAt = resp.ExpiresAt.AsTime()
	}

	return &QrisIntentResult{
		PaymentID:   resp.PaymentId,
		SnapToken:   resp.SnapToken,
		RedirectURL: resp.RedirectUrl,
		PgReference: resp.PgReference,
		ExpiresAt:   expiresAt,
	}, nil
}

// GetPayment retrieves a payment by ID from payment-service.
func (c *paymentClient) GetPayment(ctx context.Context, paymentID string) (*PaymentResult, error) {
	req := &paymentv1.GetPaymentRequest{
		Id: paymentID,
	}

	resp, err := c.client.GetPayment(ctx, req)
	if err != nil {
		return nil, err
	}

	var paidAt *time.Time
	if resp.PaidAt != nil {
		t := resp.PaidAt.AsTime()
		paidAt = &t
	}

	return &PaymentResult{
		ID:          resp.Id,
		InvoiceID:   resp.InvoiceId,
		Method:      resp.Method,
		Status:      resp.Status.String(),
		PgReference: resp.PgReference,
		AmountIDR:   resp.AmountIdr,
		CreatedAt:   resp.CreatedAt.AsTime(),
		PaidAt:      paidAt,
	}, nil
}

// Close closes the underlying gRPC connection.
func (c *paymentClient) Close() error {
	return c.conn.Close()
}
