package grpc

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	billingv1 "github.com/farid/billing-service/api/proto/billing/v1"
	"github.com/farid/billing-service/internal/billing/model"
)

// invoiceToProto translates the domain Invoice → proto Invoice.
func invoiceToProto(in *model.Invoice) *billingv1.Invoice {
	if in == nil {
		return nil
	}
	out := &billingv1.Invoice{
		Id:            in.ID,
		ReservationId: in.ReservationID,
		Status:        statusToProto(in.Status),
		TotalIdr:      in.TotalIDR,
		CreatedAt:     timestamppb.New(in.CreatedAt),
	}
	if in.ClosedAt != nil {
		out.ClosedAt = timestamppb.New(*in.ClosedAt)
	}
	if in.PaidAt != nil {
		out.PaidAt = timestamppb.New(*in.PaidAt)
	}
	out.Lines = make([]*billingv1.LineItem, 0, len(in.LineItems))
	for _, l := range in.LineItems {
		out.Lines = append(out.Lines, &billingv1.LineItem{
			Id:        l.ID,
			Kind:      lineKindToProto(l.Kind),
			AmountIdr: l.AmountIDR,
		})
	}
	return out
}

func statusToProto(s model.InvoiceStatus) billingv1.InvoiceStatus {
	switch s {
	case model.InvoiceOpen:
		return billingv1.InvoiceStatus_OPEN
	case model.InvoiceClosed:
		return billingv1.InvoiceStatus_CLOSED
	case model.InvoicePaid:
		return billingv1.InvoiceStatus_PAID
	case model.InvoiceVoid:
		return billingv1.InvoiceStatus_VOID
	}
	return billingv1.InvoiceStatus_INVOICE_STATUS_UNSPECIFIED
}

func lineKindToProto(k model.LineKind) billingv1.LineKind {
	switch k {
	case model.LineBooking:
		return billingv1.LineKind_BOOKING
	case model.LineHourly:
		return billingv1.LineKind_HOURLY
	case model.LineOvernight:
		return billingv1.LineKind_OVERNIGHT
	case model.LineCancellation:
		return billingv1.LineKind_CANCELLATION
	case model.LineNoShow:
		return billingv1.LineKind_NOSHOW
	}
	return billingv1.LineKind_LINE_KIND_UNSPECIFIED
}
