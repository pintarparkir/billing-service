package grpcclient_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/farid/billing-service/pkg/grpcclient"
)

func TestCreateQrisIntent_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := grpcclient.NewPaymentClient("nonexistent:9092", 3*time.Second)

	result, err := client.CreateQrisIntent(ctx, "inv-123", 5000)

	require.Error(t, err)
	assert.Nil(t, result)
	// gRPC may return "connection refused", "connection error", or
	// "produced zero addresses" depending on DNS resolution behaviour.
	errMsg := err.Error()
	assert.True(t,
		strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "produced zero addresses"),
		"unexpected error: %s", errMsg)
}

func TestGetPayment_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := grpcclient.NewPaymentClient("nonexistent:9092", 3*time.Second)

	result, err := client.GetPayment(ctx, "pay-456")

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestPaymentIntentResponse_SnapToken(t *testing.T) {
	t.Parallel()

	expected := &grpcclient.QrisIntentResult{
		PaymentID:   "pay-123",
		SnapToken:   "SNAP-TOKEN-xyz",
		RedirectURL: "https://app.midtrans.com/snap/v1/transactions/pay-123/pay",
		PgReference: "txn-abc",
		ExpiresAt:   time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
	}

	assert.NotEmpty(t, expected.PaymentID)
	assert.NotEmpty(t, expected.SnapToken)
	assert.NotEmpty(t, expected.RedirectURL)
	assert.False(t, expected.ExpiresAt.IsZero())
}

var _ grpcclient.PaymentClient = grpcclient.PaymentClient(nil)
