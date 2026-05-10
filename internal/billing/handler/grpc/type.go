// Package grpc adapts the BillingService proto contract to the usecase layer.
//
// The handlers here are thin: parse the request, call the usecase, map the
// domain object back to proto. Cross-cutting concerns (idempotency, logging,
// recovery, OTel) live in pkg/grpcserver interceptors and are wired in main.
package grpc

import (
	"google.golang.org/grpc"

	billingv1 "github.com/farid/billing-service/api/proto/billing/v1"
	"github.com/farid/billing-service/internal/billing/usecase"
)

type Server struct {
	billingv1.UnimplementedBillingServiceServer
	uc usecase.BillingUsecase
}

// Register wires this Server into a gRPC ServiceRegistrar.
func Register(s grpc.ServiceRegistrar, uc usecase.BillingUsecase) {
	billingv1.RegisterBillingServiceServer(s, &Server{uc: uc})
}
