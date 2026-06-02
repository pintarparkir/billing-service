# billing-service

[![Security](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_billing-service&metric=security_rating&token=627302b199b2f792bd1ba8dc5a91777d38398d2c)](https://sonarcloud.io/summary/new_code?id=pintarparkir_billing-service)
[![Reliability](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_billing-service&metric=reliability_rating&token=627302b199b2f792bd1ba8dc5a91777d38398d2c)](https://sonarcloud.io/summary/new_code?id=pintarparkir_billing-service)
[![Maintainability](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_billing-service&metric=sqale_rating&token=627302b199b2f792bd1ba8dc5a91777d38398d2c)](https://sonarcloud.io/summary/new_code?id=pintarparkir_billing-service)
[![Duplications](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_billing-service&metric=duplicated_lines_density&token=627302b199b2f792bd1ba8dc5a91777d38398d2c)](https://sonarcloud.io/summary/new_code?id=pintarparkir_billing-service)

**Cloud Run:** `https://billing-service-725nddkmwq-as.a.run.app`

## Architecture Overview

![Architecture](docs/PintarParkir.architecture.svg)

## E2E Flow

![Flow Diagram](docs/flow.diagram.svg)

## Sequence Diagrams

![E2E Sequence Diagram](docs/sequence-diagram.png)

---



> **Purpose:** Invoice lifecycle management — owns invoice ledger, pricing engine, cancel/no-show fees, and reconciliation.  
> **Author:** Farid Triwicaksono · **Last Updated:** 2026-05-21

## Project Overview

**ParkirPintar** is a backend mini-app for smart parking within a super-app. It handles:
- Availability queries (spots per floor, per vehicle type)
- Reservation creation (system-assigned or user-selected spots)
- Reservation state transitions (confirm, cancel, check-in, check-out)
- Geofence validation (GPS-based check-in)
- No-show expiration (automatic after 1 hour hold)
- Event publishing (outbox pattern → RabbitMQ)

Five services: **user**, **reservation**, **billing** (this service), **payment**, **notification**.

## Service Scope

**Owns:**
- Invoice ledger (OPEN → CLOSED → PAID lifecycle)
- Pricing engine (pure-functional, no I/O)
- Line items (booking fee, hourly rate, overnight flat, cancel fee, no-show fee)
- Invoice closing logic (triggered by reservation check-out or expiration)
- Reconciliation cron (scans reservations > 5min without invoice, re-publishes event)
- Outbox event publishing (`invoice.closed.v1`)

**Does NOT own:**
- Reservation state (reservation-service owns)
- Payment processing (payment-service owns)
- Driver identity (user-service owns)
- SMS dispatch (notification-service owns)

**Key invariants:**
- One invoice per reservation (`reservation_id` unique)
- Invoice immutable after CLOSED (append-only line items)
- Pricing engine deterministic (same session → same total)
- Idempotency via reservation_id natural key

## At a Glance

| Aspect | Details |
|--------|---------|
| **REST Port** | N/A (gRPC only) |
| **gRPC Port** | 9091 (s2s — called by reservation/payment) |
| **Database** | PostgreSQL 16 (invoice, line_item, outbox_event) |
| **Cache** | N/A |
| **Message Queue** | RabbitMQ 3.13 (consumer: reservation.*, producer: invoice.closed.v1) |
| **External APIs** | None |

## Tech Stack

- **Language:** Go 1.22
- **Web Framework:** gRPC only
- **Database:** PostgreSQL 16 + sqlx
- **Message Queue:** RabbitMQ 3.13 (amqp091-go)
- **Logging:** Zap + Lumberjack
- **Observability:** OpenTelemetry (OTLP/gRPC)
- **Testing:** testify/mock, table-driven tests
- **Pricing Engine:** Pure-functional (`pkg/pricing`)

## Architecture

### High-Level Design
See [`../docs/architecture/high-level-design/03-billing-service.md`](../docs/architecture/high-level-design/03-billing-service.md) for:
- Service responsibilities and boundaries
- Async event consumption (`reservation.created.v1`, `reservation.cancelled.v1`, `reservation.expired.v1`, `reservation.checked_out.v1`)
- Invoice lifecycle state machine

### Low-Level Design
See [`../docs/architecture/low-level-design/03-billing-service-lld.md`](../docs/architecture/low-level-design/03-billing-service-lld.md) for:
- Layer cake (model → usecase → repository → handler)
- Pricing engine invocation (pure function)
- Transaction boundaries (invoice close + line items + outbox in single tx)

### Entity Relationship Diagram
See [`../docs/architecture/erd/03-billing-service.md`](../docs/architecture/erd/03-billing-service.md) for:
- Table schema (invoice, line_item, outbox_event, idempotency_key)
- Unique constraint (`reservation_id`)
- Critical indexes (`driver_id`, `status`, `created_at`)

![ParkirPintar ERD](../user-service/ERD.jpg)

## API Reference

### gRPC Services (s2s, internal only)

| RPC | Input | Output | Purpose |
|-----|-------|--------|---------|
| OpenInvoice | OpenInvoiceRequest | Invoice | Create invoice on reservation creation (idempotent on reservation_id) |
| CloseInvoice | CloseInvoiceRequest | Invoice | Close invoice on check-out (applies pricing engine) |
| GetInvoice | GetInvoiceRequest | Invoice | Lookup invoice by ID or reservation_id |
| ListInvoices | ListInvoicesRequest | ListInvoicesResponse | Admin list with pagination |

### RabbitMQ Events

**Consumes:**
| Event | Trigger | Action |
|-------|---------|--------|
| `reservation.created.v1` | Reservation created | OpenInvoice (idempotent) |
| `reservation.cancelled.v1` | Reservation cancelled | Apply cancel fee if > grace period |
| `reservation.expired.v1` | No-show after 1h | Apply no-show fee + close invoice |
| `reservation.checked_out.v1` | Check-out succeeds | CloseInvoice (apply pricing engine) |

**Produces:**
| Event | Trigger | Payload |
|-------|---------|---------|
| `invoice.closed.v1` | Invoice closed | invoice_id, reservation_id, driver_id, total_idr, closed_at |

## Sample Environment

```bash
# ── App ─────────────────────────────────────────────────────────────────────
APP_NAME=billing-service
APP_ENV=local
GRPC_PORT=9091

# ── Postgres ────────────────────────────────────────────────────────────────
DB_HOST=localhost
DB_PORT=5432
DB_USERNAME=postgres
DB_PASSWORD=postgres
DB_NAME=billing_service

# ── RabbitMQ (event consumption + publishing) ───────────────────────────────
RABBIT_URL=amqp://guest:guest@localhost:5672/
RABBIT_EXCHANGE=parkirpintar.events

# ── Observability ────────────────────────────────────────────────────────────
OTLP_ENDPOINT=localhost:4317

# ── Pricing tariffs (IDR) ───────────────────────────────────────────────────
BOOKING_FEE_IDR=5000
HOURLY_RATE_IDR=5000
OVERNIGHT_FLAT_IDR=20000
CANCEL_FEE_IDR=5000
NO_SHOW_FEE_IDR=5000
CANCEL_GRACE_MINUTES=2
```

See `configs/.env.example` for full reference.

## Getting Started

### Prerequisites
- Docker 24+ & Docker Compose v2
- Go 1.22+ (for local development)
- `buf` CLI (for proto regeneration)

### Local Development

```bash
# 1. Clone and setup
git clone <repo> && cd <repo>
cd billing-service
cp configs/.env.example configs/.env

# 2. Start shared infra (see https://github.com/pintarparkir/infra)
cd ../infra && podman compose up -d

# 3. Run migrations
cd ../billing-service
make migrate-up

# 4. Run the service
make run

# 5. Verify health
grpcurl -plaintext localhost:9091 grpc.health.v1.Health/Check
```

## Testing

### Unit Tests (no infra)
```bash
make test-unit
```
Covers: pricing engine (pure functions), usecase logic, fee calculation.

### All Tests
```bash
make test
```

### Coverage
```bash
go test -coverprofile=cov.out ./...
go tool cover -html=cov.out
```
Target: usecase ≥80%, repository ≥60%.

## Debugging

### Logs
```bash
LOG_LEVEL=debug make run
```
Logs are JSON-formatted with trace_id, span_id, request_id.

### Database
```bash
psql postgresql://postgres:postgres@localhost:5432/billing_service

# View schema
\dt

# Check invoice status
SELECT id, reservation_id, status, total_idr FROM invoice ORDER BY created_at DESC LIMIT 10;

# Check line items
SELECT invoice_id, line_type, amount_idr, metadata FROM line_item WHERE invoice_id = '<invoice_id>';
```

### RabbitMQ
- **Management UI:** http://localhost:15672 (guest/guest)
- **View exchange:** parkirpintar.events
- **View queues:** billing.reservation.* queues
- **Inspect DLQ:** billing.reservation.dlq

### gRPC
```bash
# Test gRPC health
grpcurl -plaintext localhost:9091 grpc.health.v1.Health/Check

# Call OpenInvoice
grpcurl -plaintext -d '{"reservation_id":"<id>","driver_id":"<driver_id>"}' \
  localhost:9091 parkirpintar.billing.v1.BillingService/OpenInvoice
```

## Operations

### Health Checks
```bash
grpcurl -plaintext localhost:9091 grpc.health.v1.Health/Check
```

### Migrations
```bash
make migrate-up      # Apply all pending migrations
make migrate-down    # Rollback one migration
```

### Outbox Publisher
Background worker publishes unsent outbox events to RabbitMQ every 5 seconds. Check logs for `outbox: published` messages.

### Reconciliation Cron
Background worker scans reservations older than 5 minutes without invoice every 5 minutes and re-publishes `reservation.created.v1`.

## Security Notes

- **Secrets:** Never commit `.env` files. Use Secret Manager in production.
- **SQL:** All queries parameterized (sqlx prevents injection).
- **Idempotency:** Scoped per reservation_id to prevent duplicate invoices.
- **Pricing engine:** Pure-functional, no side effects, fully deterministic.

## Business Flow Logic

### Billing Service in Reservation Lifecycle

Billing-service adalah **event consumer** yang bereaksi terhadap perubahan state reservation. Service ini tidak memiliki flow bisnis mandiri yang di-trigger langsung oleh user.

```mermaid
sequenceDiagram
    autonumber
    participant RMQ as RabbitMQ
    participant Billing as Billing Service
    participant Pricing as Pricing Engine
    participant DB as Postgres DB
    participant Notif as Notification Service
    
    Note over RMQ,Notif: Event 1: Reservation Created → Open Invoice
    
    RMQ-->Billing: CONSUME reservation.created.v1
    activate Billing
    
    Billing->>DB: BEGIN
    Billing->>DB: SELECT * FROM invoice WHERE reservation_id = ?
    
    alt Invoice not exists
        Billing->>DB: INSERT INTO invoice (<br/>reservation_id, driver_id,<br/>status='OPEN')
        Billing->>DB: INSERT INTO invoice_line (<br/>kind='BOOKING', amount=2000)
        Billing->>DB: COMMIT
        Note right of Billing: Idempotent on reservation_id
    else Invoice exists (replay)
        Billing->>DB: COMMIT (skip)
    end
    
    deactivate Billing
    
    Note over RMQ,Notif: Event 2: Check-out → Close Invoice & Calculate
    
    RMQ-->Billing: CONSUME reservation.checked_out.v1
    activate Billing
    
    Billing->>DB: SELECT * FROM invoice WHERE reservation_id = ? FOR UPDATE
    Billing->>DB: SELECT checked_in_at, checked_out_at FROM reservation
    
    %% Pricing engine invocation
    Billing->>Pricing: CalculatePricing({<br/>checked_in: 08:00,<br/>checked_out: 10:30,<br/>vehicle_type: 'CAR'})
    activate Pricing
    Note right of Pricing: Pure functional<br/>No time.Now() calls<br/>Deterministic output
    
    Pricing->>Pricing: Duration: 2.5 hours
    Pricing->>Pricing: BOOKING fee: 2,000 (flat)
    Pricing->>Pricing: HOURLY fee: 12,500 (2.5h × 5,000)
    Pricing->>Pricing: No OVERNIGHT (within 06:00-22:00)
    
    Pricing-->>Billing: {total: 14500, lines: [...]}
    deactivate Pricing
    
    Billing->>DB: BEGIN
    Billing->>DB: UPDATE invoice SET<br/>total_idr=14500, status='CLOSED'
    Billing->>DB: INSERT INTO invoice_line (<br/>kind='HOURLY', amount=12500)
    Billing->>DB: INSERT INTO outbox_event(<br/>topic='billing.invoice.closed.v1')
    Billing->>DB: COMMIT
    
    Billing->>RMQ: PUBLISH billing.invoice.closed.v1
    RMQ-->Notif: CONSUME → Send SMS receipt
    
    deactivate Billing
    
    Note over RMQ,Notif: Event 3: Cancellation → Apply Cancel Fee
    
    RMQ-->Billing: CONSUME reservation.cancelled.v1
    activate Billing
    
    Billing->>Billing: CalculateCancelFee(<br/>cancelled_at: 08:00,<br/>hold_expiry: 08:10)
    
    alt Cancelled within grace period (30min)
        Billing->>DB: INSERT INTO invoice_line (<br/>kind='CANCELLATION', amount=2000)
        Note right of Billing: Booking fee retained as penalty
    else Early cancellation
        Billing->>DB: No fee (free cancel)
    end
    
    Billing->>DB: UPDATE invoice SET status='CLOSED'
    Billing->>DB: COMMIT
    
    deactivate Billing
    
    Note over RMQ,Notif: Event 4: No-show → Apply No-show Fee
    
    RMQ-->Billing: CONSUME reservation.expired.v1
    activate Billing
    
    Billing->>DB: BEGIN
    Billing->>DB: INSERT INTO invoice_line (<br/>kind='NOSHOW', amount=5000)
    Billing->>DB: UPDATE invoice SET status='CLOSED',<br/>total = booking + noshow
    Billing->>DB: COMMIT
    
    deactivate Billing
```

### Pricing Engine Rules

| Fee Type | Condition | CAR Rate | MOTORCYCLE Rate |
|----------|-----------|----------|-----------------|
| **BOOKING** | Always (flat) | IDR 2,000 | IDR 1,000 |
| **HOURLY** | After check-out | IDR 5,000/h | IDR 2,000/h |
| **OVERNIGHT** | 22:00-06:00 | IDR 30,000 | IDR 15,000 |
| **CANCELLATION** | Within 30min of hold expiry | Booking fee retained | Booking fee retained |
| **NOSHOW** | Reservation expired | IDR 5,000 penalty | IDR 2,500 penalty |

### Invoice Lifecycle

```
┌──────────┐    reservation.created.v1     ┌──────────┐
│  (none)  │ ────────────────────────────▶ │   OPEN   │
└──────────┘                               └────┬─────┘
                                                │
                      ┌─────────────────────────┼─────────────────────────┐
                      │                         │                         │
                      ▼                         ▼                         ▼
         reservation.checked_out.v1    reservation.cancelled.v1   reservation.expired.v1
                      │                         │                         │
                      ▼                         ▼                         ▼
               ┌────────────┐            ┌────────────┐            ┌────────────┐
               │   CLOSED   │            │   CLOSED   │            │   CLOSED   │
               │ (paid due) │            │(cancel fee)│            │(noshow fee)│
               └────────────┘            └────────────┘            └────────────┘
```

---

## Related Documentation

- **Architecture Overview:** [`../docs/README.md`](../docs/README.md)
- **Shared Infra Docs:** [`infra`](https://github.com/pintarparkir/infra)
- **API Documentation:** [`../docs/api-documentation/00-overview.md`](../docs/api-documentation/00-overview.md)
- **Implementation Backlog:** [`../docs/implementation-todo/00-backlog.md`](../docs/implementation-todo/00-backlog.md)

---

_For questions or issues, refer to the troubleshooting section in the main README or open an issue on the repo._
