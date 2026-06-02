package http

import (
	"encoding/json"
	"net/http"

	"github.com/farid/billing-service/internal/billing/usecase"
)

// InvoiceHandler exposes invoice queries over HTTP.
type InvoiceHandler struct {
	uc usecase.BillingUsecase
}

func NewInvoiceHandler(uc usecase.BillingUsecase) *InvoiceHandler {
	return &InvoiceHandler{uc: uc}
}

// GetByReservation handles GET /v1/invoices?reservation_id=xxx
func (h *InvoiceHandler) GetByReservation(w http.ResponseWriter, r *http.Request) {
	resID := r.URL.Query().Get("reservation_id")
	if resID == "" {
		http.Error(w, `{"error":"reservation_id required"}`, http.StatusBadRequest)
		return
	}
	inv, err := h.uc.GetInvoiceByReservation(r.Context(), resID)
	if err != nil {
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":             inv.ID,
		"reservation_id": inv.ReservationID,
		"total_idr":      inv.TotalIDR,
		"status":         inv.Status,
	})
}
