package usecase

import (
	"context"
	"encoding/json"

	"github.com/farid/billing-service/internal/billing/model"
	"github.com/farid/billing-service/pkg/pricing"
)

func (u *billingUsecase) CloseInvoice(ctx context.Context, invoiceID string, session pricing.Session) (*model.Invoice, error) {
	// Run the pricing engine purely (no I/O in this call).
	rawLines := u.engine.Apply(session)

	// Translate pricing.LineItem → model.LineItem and drop the synthetic 0-IDR
	// CANCELLATION line the engine uses internally for grace cancels.
	dbLines := make([]model.LineItem, 0, len(rawLines))
	for _, l := range rawLines {
		if l.Kind == pricing.LineCancellation && l.AmountIDR == 0 {
			continue
		}
		dbLines = append(dbLines, model.LineItem{
			Kind:      model.LineKind(l.Kind),
			AmountIDR: l.AmountIDR,
			Metadata:  map[string]any{"note": l.Note},
		})
	}

	payload, _ := json.Marshal(map[string]any{
		"invoice_id": invoiceID,
		"lines":      rawLines,
	})
	return u.repo.Close(ctx, invoiceID, dbLines, payload)
}

func (u *billingUsecase) GetInvoice(ctx context.Context, id string) (*model.Invoice, error) {
	return u.repo.GetByID(ctx, id)
}

func (u *billingUsecase) GetInvoiceByReservation(ctx context.Context, reservationID string) (*model.Invoice, error) {
	return u.repo.GetByReservationID(ctx, reservationID)
}
