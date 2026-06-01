// Package http provides HTTP handlers for billing-service, including payment gateway webhooks.
package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/farid/billing-service/internal/billing/usecase"
	"github.com/farid/billing-service/pkg/logger"
)

// WebhookHandler handles payment gateway webhook notifications.
// Currently supports Midtrans-like callback payloads.
type WebhookHandler struct {
	uc     usecase.BillingUsecase
	signKey string
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(uc usecase.BillingUsecase, signKey string) *WebhookHandler {
	return &WebhookHandler{
		uc:      uc,
		signKey: signKey,
	}
}

// Handle processes payment gateway callbacks.
// Expected payload format (Midtrans-like):
// {
//   "order_id": "PAY-ref-xxx",
//   "transaction_status": "settlement|capture|cancel|deny|expire|pending",
//   "fraud_status": "accept|challenge|deny",
//   "gross_amount": "5000",
//   "signature_key": "abc123..."
// }
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := readBody(r)
	if err != nil {
		logger.Error(r.Context(), "webhook: failed to read body", map[string]interface{}{"error": err.Error()})
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Verify signature before processing
	var webhookMap map[string]interface{}
	if err := json.Unmarshal(body, &webhookMap); err == nil {
		if sigKey, ok := webhookMap["signature_key"].(string); ok && sigKey != "" {
			if !verifySignature(sigKey, h.signKey, body) {
				logger.Error(r.Context(), "webhook: invalid signature", map[string]interface{}{"order_id": getOrderId(webhookMap)})
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}
	}

	// Process notification through usecase
	ctx := r.Context()
	if err := h.uc.HandlePaymentNotification(ctx, body); err != nil {
		logger.Error(ctx, "webhook: failed to process notification", map[string]interface{}{"error": err.Error()})
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	buf := make([]byte, 4096) // Max 4KB webhook payload
	n, err := r.Body.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return nil, err
	}
	return buf[:n], nil
}

func verifySignature(sig, key string, body []byte) bool {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

func getOrderId(data map[string]interface{}) string {
	if orderID, ok := data["order_id"].(string); ok {
		return orderID
	}
	if paymentRef, ok := data["payment_ref"].(string); ok {
		return paymentRef
	}
	return ""
}
