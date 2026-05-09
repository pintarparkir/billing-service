# High-Level Design — billing-service

Internal-only ledger service. No public REST surface. Driven by:
- Synchronous gRPC from reservation-service (open / close / read invoice).
- Async RabbitMQ events (cancel-fee on `reservation.cancelled.v1`,
  no-show fee on `reservation.expired.v1`).

## Position in the system

```
                          reservation-service
                                 │
                       gRPC :9091│
                                 ▼
   RabbitMQ ─consumer──▶  billing-service ──producer──▶ RabbitMQ
   (reservation.*.v1)         │      │        (billing.invoice.*.v1)
                              │      │
                              ▼      ▼
                          Postgres (invoice + invoice_line)
```

## Responsibilities

- **Invoice ledger** — append-only line-item ledger; invoice-level totals derived
  from `SUM(line.amount_idr)`.
- **Pricing engine** — pure functional rules in `pkg/pricing` (testable without I/O).
- **Idempotent OpenInvoice** — `Idempotency-Key` from gRPC metadata, stored on
  invoice row.
- **Event-driven cancel-fee / no-show-fee** — subscribes to reservation events and
  appends the relevant line.
- **Emits** `billing.invoice.opened.v1` and `billing.invoice.closed.v1` for
  payment + notification consumers.

## Sequence — Close on check-out

```
reservation-svc      billing-svc                 Postgres        RabbitMQ
      │── gRPC CloseInvoice() ──▶│                  │              │
      │                          │── load invoice ─▶│              │
      │                          │── apply pricing engine (pure) ──│
      │                          │── INSERT lines ─▶│              │
      │                          │── UPDATE invoice CLOSED ─▶      │
      │                          │── INSERT outbox_event ──▶│      │
      │◀── Invoice ──────────────│                  │              │
      │                          │   outbox-publisher loop         │
      │                          │   ── publish billing.invoice.closed.v1 ──▶
```

## Why no REST surface

billing is an implementation detail behind reservation. The mini app only sees
invoice IDs/totals through reservation responses. If the mini app needs to fetch
an invoice directly later, the path is `reservation-service /v1/reservations/{id}`
embedding the invoice — *not* a billing REST endpoint.

This keeps billing's contract narrow and test-friendly.
