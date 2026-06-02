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
	// Handle both /v1/invoices?reservation_id=xxx and /v1/invoices/{id}
	path := r.URL.Path
	if path != "/v1/invoices" {
		// Extract ID from path: /v1/invoices/{id}
		id := path[len("/v1/invoices/"):]
		if id != "" {
			h.getByID(w, r, id)
			return
		}
	}
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

func (h *InvoiceHandler) getByID(w http.ResponseWriter, r *http.Request, id string) {
	inv, err := h.uc.GetInvoice(r.Context(), id)
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
