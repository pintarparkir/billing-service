package grpc

import (
	"context"
	"time"

	billingv1 "github.com/farid/billing-service/api/proto/billing/v1"
	"github.com/farid/billing-service/pkg/pricing"
	"github.com/farid/billing-service/pkg/utils"
)

// OpenInvoice is idempotent on the Idempotency-Key gRPC metadata header.
// The interceptor in pkg/grpcserver guards replay caching at the wire level;
// the usecase also accepts the key as the row-level idempotency_key so a
// stale interceptor cache still de-dupes against the existing OPEN row.
func (s *Server) OpenInvoice(ctx context.Context, req *billingv1.OpenInvoiceRequest) (*billingv1.Invoice, error) {
	idem := utils.IdempotencyKeyFromCtx(ctx)
	inv, err := s.uc.OpenInvoice(ctx, req.GetReservationId(), req.GetDriverId(), idem)
	if err != nil {
		return nil, err // pkg/grpcserver mapError converts AppError → status code
	}
	return invoiceToProto(inv), nil
}

// CloseInvoice runs the pricing engine using the timestamps in the request.
// Caller (reservation-service) supplies confirmed_at / checked_in_at /
// checked_out_at — the engine is pure and infers everything else.
func (s *Server) CloseInvoice(ctx context.Context, req *billingv1.CloseInvoiceRequest) (*billingv1.Invoice, error) {
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	session := pricing.Session{
		CheckedInAt:  asTime(req.GetCheckedInAt().AsTime()),
		CheckedOutAt: asTime(req.GetCheckedOutAt().AsTime()),
		// Reservation hasn't included confirmed_at on the proto yet; the
		// engine doesn't strictly need it for the hourly path. If we add
		// cancel-via-gRPC later, surface it as a separate field.
		Timezone:    jakarta,
		VehicleType: pricing.VehicleCar,
	}
	inv, err := s.uc.CloseInvoice(ctx, req.GetInvoiceId(), session)
	if err != nil {
		return nil, err
	}
	return invoiceToProto(inv), nil
}

func (s *Server) GetInvoice(ctx context.Context, req *billingv1.GetInvoiceRequest) (*billingv1.Invoice, error) {
	inv, err := s.uc.GetInvoice(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return invoiceToProto(inv), nil
}

// asTime preserves a zero-value Time when the request didn't include the field
// (timestamppb.AsTime() returns 1970-01-01 in that case, which the engine
// would mistreat as a real input).
func asTime(t time.Time) time.Time {
	if t.Year() <= 1971 {
		return time.Time{}
	}
	return t
}
