# billing-service

Invoice ledger + pricing engine. Internal-only — no public REST surface.
Driven by reservation-service over gRPC and by RabbitMQ events.

## At a glance

| Surface | Port  | Used by                                       |
|---------|-------|-----------------------------------------------|
| gRPC    | 9091  | reservation-service (`OpenInvoice`, `CloseInvoice`) |
| RabbitMQ| —     | consumer of `reservation.*.v1` events         |

## gRPC API

| RPC            | Description                                              |
|----------------|----------------------------------------------------------|
| OpenInvoice    | Create OPEN invoice with booking-fee line (idempotent)   |
| CloseInvoice   | Apply pricing engine, set total, emit closed event       |
| GetInvoice     | Read by id                                               |

## Pricing rules (`pkg/pricing`)

Pure-functional — no I/O, fully unit-testable. See `docs/features/03-pricing-engine.md`.

| Rule           | Trigger                                            |
|----------------|----------------------------------------------------|
| BookingFee     | Always on `OpenInvoice`, 5,000 IDR                 |
| Hourly         | `ceil(duration_min / 60) * 5,000`                  |
| Overnight      | If session crosses 00:00 WIB → flat 20,000 IDR     |
| Cancellation   | 0 IDR within 2-min grace, 5,000 IDR after          |
| NoShow         | 5,000 IDR on `reservation.expired.v1`              |

## Service dependencies

| Dependency | Protocol | Purpose                                |
|------------|----------|----------------------------------------|
| RabbitMQ   | AMQP     | Consume reservation events; emit billing events |
| PostgreSQL | TCP      | Invoice + line-item ledger             |

## Run

```bash
cd ../infra && docker compose up -d
cd ../billing-service
cp configs/.env.example configs/.env
make migrate-up
make run
```

## Docs

- `docs/architecture/high-level-design.md`
- `docs/architecture/erd.md` — `invoice`, `invoice_line`, `outbox_event`
- `docs/features/` — per-feature specs
