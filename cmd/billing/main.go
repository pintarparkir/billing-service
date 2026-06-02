// billing-service entry point.
//
// Wires:
//   - configs → logger → otel
//   - postgres + rabbitmq publisher + subscriber
//   - repository (invoice + outbox + payment_requests)
//   - pricing engine (pure)
//   - usecase
//   - background workers (outbox publisher)
//   - RabbitMQ consumer (reservation events + payment events)
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
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/farid/billing-service/internal/billing/consumer"
	billgrpc "github.com/farid/billing-service/internal/billing/handler/grpc"
	billhttp "github.com/farid/billing-service/internal/billing/handler/http"
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
	paymentRepo := billrepo.NewPaymentRequestRepository(db)
	uc := billuc.NewBillingUsecase(repo, engine, pricingCfg).WithPaymentRequestRepository(paymentRepo)

	// ── gRPC server (s2s callers — reservation-service.CreatePaymentRequest) ─
	grpcSrv, _ := grpcserver.NewGrpcServerNoListen(grpcserver.Options{
		IdempotencyStore:  idempotency.NewPostgresStore(db),
		IdempotentMethods: []string{model.ScopeOpenInvoice},
	})
	billgrpc.Register(grpcSrv.Server, uc)

	// ── HTTP server (multiplexed gRPC+REST on same port via h2c) ─────────────
	webhookHandler := billhttp.NewWebhookHandler(uc, "") // TODO: add config for Midtrans signing key
	invoiceHandler := billhttp.NewInvoiceHandler(uc)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			logger.Error(context.Background(), "health write failed", map[string]interface{}{logger.ErrorKey: err.Error()})
		}
	})
	mux.HandleFunc("/v1/invoices", invoiceHandler.GetByReservation)
	mux.HandleFunc("/v1/invoices/", invoiceHandler.GetByReservation)
	mux.HandleFunc("/webhook/payment", webhookHandler.Handle)

	var protos http.Protocols
	protos.SetHTTP1(true)
	protos.SetUnencryptedHTTP2(true)
	httpServer := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      grpcHTTPMux(grpcSrv.Server, mux),
		Protocols:    &protos,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	go func() {
		logger.Info(ctx, "billing-service listening (gRPC+HTTP)", map[string]interface{}{"port": cfg.AppPort})
		if err := httpServer.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
			logger.Error(ctx, "listen failed", map[string]interface{}{logger.ErrorKey: err.Error()})
		}
	}()

	// ── Background workers ───────────────────────────────────────────────────
	go worker.NewOutboxPublisher(obRepo, pub).Run(ctx)

	// ── RabbitMQ consumer (reservation events + payment confirmation) ─────────
	c := consumer.NewReservation(uc)
	go func() {
		logger.Info(ctx, "consumer: subscribing", map[string]interface{}{
			"queue": cfg.RabbitQueue,
			"keys":  "reservation.cancelled.v1, reservation.expired.v1, reservation.checked_out.v1, billing.payment.success.v1, billing.payment.failed.v1",
		})
		if err := sub.Consume(ctx, c.Handle); err != nil {
			logger.Error(ctx, "consumer: stopped", map[string]interface{}{logger.ErrorKey: err.Error()})
		}
	}()

	// ── Graceful shutdown ────────────────────────────────────────────────────
	<-ctx.Done()
	logger.Info(context.Background(), "shutdown signal received", nil)
	grpcSrv.Server.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	if err := logger.Sync(); err != nil {
		fmt.Fprintln(os.Stderr, "logger sync:", err)
	}
}

// grpcHTTPMux routes gRPC to grpcServer, everything else to httpHandler.
func grpcHTTPMux(grpcSrv *grpc.Server, httpHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpcSrv.ServeHTTP(w, r)
			return
		}
		httpHandler.ServeHTTP(w, r)
	})
}
