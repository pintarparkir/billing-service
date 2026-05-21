// billing-service entry point.
//
// Wires:
//   - configs → logger → otel
//   - postgres + rabbitmq publisher + subscriber
//   - repository (invoice + outbox)
//   - pricing engine (pure)
//   - usecase
//   - background workers (outbox publisher)
//   - RabbitMQ consumer (reservation events → cancel/no-show/close)
//
// gRPC server registration is conditional on `buf generate` having produced
// api/proto/billing/v1/*.pb.go. Until then the service runs only as an
// event-driven worker (no inbound gRPC). All business logic is reachable via
// the consumer + outbox publisher path; the gRPC handler files are written
// but not registered. See docs/features/01-open-invoice.md.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/farid/billing-service/internal/billing/consumer"
	billgrpc "github.com/farid/billing-service/internal/billing/handler/grpc"
	billrepo "github.com/farid/billing-service/internal/billing/repository/postgres"
	billuc "github.com/farid/billing-service/internal/billing/usecase"
	"github.com/farid/billing-service/internal/billing/worker"

	"github.com/farid/billing-service/internal/billing/model"
	"github.com/farid/billing-service/pkg/configs"
	pgdb "github.com/farid/billing-service/pkg/db/postgres"
	"github.com/farid/billing-service/pkg/grpcserver"
	"github.com/farid/billing-service/pkg/idempotency"
	"github.com/farid/billing-service/pkg/logger"
	pkgOtel "github.com/farid/billing-service/pkg/otel"
	"github.com/farid/billing-service/pkg/pricing"
	"github.com/farid/billing-service/pkg/rabbit"
)

func main() {
	cfg := configs.NewConfig(configs.ConfigLoader{Env: os.Getenv("PROJECT_ENV")})
	if err := logger.NewLogger(cfg.AppName, cfg.AppEnv); err != nil {
		panic(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	otel := pkgOtel.NewOpenTelemetry(cfg.OTLPEndpoint, "billing", cfg.AppEnv)
	defer func() {
		if err := otel.EndAPM(); err != nil {
			fmt.Fprintln(os.Stderr, "otel shutdown:", err)
		}
	}()

	// ── Infra ────────────────────────────────────────────────────────────────
	db, err := pgdb.NewPostgresDB(pgdb.PostgresDsn{
		Host: cfg.DbHost, Port: cfg.DbPort, User: cfg.DbUsername, Password: cfg.DbPassword, Db: cfg.DbName,
		MaxOpen: cfg.DbMaxOpen, MaxIdle: cfg.DbMaxIdle,
	})
	if err != nil {
		logger.Fatal(ctx, "postgres init failed", map[string]interface{}{logger.ErrorKey: err.Error()})
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			logger.Error(ctx, "db.Close failed", map[string]interface{}{logger.ErrorKey: closeErr.Error()})
		}
	}()

	pub, err := rabbit.NewPublisher(cfg.RabbitURL, cfg.RabbitExchange)
	if err != nil {
		logger.Fatal(ctx, "rabbitmq publisher init failed", map[string]interface{}{logger.ErrorKey: err.Error()})
	}
	defer pub.Close()

	sub, err := rabbit.NewSubscriber(cfg.RabbitURL, cfg.RabbitExchange, cfg.RabbitQueue,
		[]string{
			model.EvtReservationCreated,
			model.EvtReservationCancelled,
			model.EvtReservationExpired,
			model.EvtReservationCheckedOut,
		})
	if err != nil {
		logger.Fatal(ctx, "rabbitmq subscriber init failed", map[string]interface{}{logger.ErrorKey: err.Error()})
	}
	defer sub.Close()

	// ── Domain wiring ────────────────────────────────────────────────────────
	pricingCfg := pricing.Config{
		BookingFeeIDR:    cfg.BookingFeeIDR,
		HourlyRateIDR:    cfg.HourlyRateIDR,
		OvernightFlatIDR: cfg.OvernightFlatIDR,
		CancelFeeIDR:     cfg.CancelFeeIDR,
		NoShowFeeIDR:     cfg.NoShowFeeIDR,
		CancelGrace:      time.Duration(cfg.CancelGraceMin) * time.Minute,
	}
	engine := pricing.NewDefaultEngine(pricingCfg)

	repo := billrepo.NewInvoiceRepository(db)
	obRepo := billrepo.NewOutboxRepository(db)
	uc := billuc.NewBillingUsecase(repo, engine, pricingCfg)

	// ── gRPC server (s2s callers — reservation-service.OpenInvoice/CloseInvoice) ─
	// CloseInvoice is naturally idempotent at the row level (status != OPEN
	// short-circuits) and reservation-service doesn't have a stable client key
	// to pass on the close path, so we don't enforce header-level idempotency
	// here. OpenInvoice still requires Idempotency-Key from the caller.
	grpcSrv, err := grpcserver.NewGrpcServer(cfg.GrpcPort, grpcserver.Options{
		IdempotencyStore:  idempotency.NewPostgresStore(db),
		IdempotentMethods: []string{model.ScopeOpenInvoice},
	})
	if err != nil {
		logger.Fatal(ctx, "grpc server init failed", map[string]interface{}{logger.ErrorKey: err.Error()})
	}
	billgrpc.Register(grpcSrv.Server, uc)
	go func() {
		if err := grpcSrv.Start(); err != nil {
			logger.Fatal(ctx, "grpc serve failed", map[string]interface{}{logger.ErrorKey: err.Error()})
		}
	}()

	// ── Background workers ───────────────────────────────────────────────────
	go worker.NewOutboxPublisher(obRepo, pub).Run(ctx)

	// ── RabbitMQ consumer (reservation events) ───────────────────────────────
	c := consumer.NewReservation(uc)
	go func() {
		logger.Info(ctx, "consumer: subscribing", map[string]interface{}{
			"queue": cfg.RabbitQueue,
			"keys":  "reservation.cancelled.v1, reservation.expired.v1, reservation.checked_out.v1",
		})
		if err := sub.Consume(ctx, c.Handle); err != nil {
			logger.Error(ctx, "consumer: stopped", map[string]interface{}{logger.ErrorKey: err.Error()})
		}
	}()

	// ── Graceful shutdown ────────────────────────────────────────────────────
	<-ctx.Done()
	logger.Info(context.Background(), "shutdown signal received", nil)
	grpcSrv.Shutdown()
	if err := logger.Sync(); err != nil {
		fmt.Fprintln(os.Stderr, "logger sync:", err)
	}
}
