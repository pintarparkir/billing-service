# Feature 04 — Cancel-fee / no-show-fee consumer

**Status:** ✅ shipped
**Owner:** billing-service

## Scope

Subscribe to RabbitMQ events from reservation-service that imply a charge
(cancellation, no-show). Append the corresponding line to the open invoice.

| Routing key                 | Action                                          |
|-----------------------------|-------------------------------------------------|
| `reservation.cancelled.v1`  | If outside 2-min grace → append CANCELLATION line, close invoice |
| `reservation.expired.v1`    | Append NOSHOW line, close invoice               |

## Why event-driven, not synchronous

reservation-service's cancel path stays fast (one DB tx + outbox row). Billing
catches up asynchronously. Worst case: a cancel returns 200 to the user before
the fee line appears in the invoice — that's fine, the invoice is still OPEN.

## Idempotency

Each event payload includes the `reservation_id`. Consumer logic:

```
SELECT id, status FROM invoice WHERE reservation_id = $1
if status != OPEN → no-op (already closed, idempotent on retry)
INSERT invoice_line ON CONFLICT DO NOTHING
  (uniqueness on (invoice_id, kind) for these one-off fee lines)
```

## Tasks

- [ ] `internal/billing/consumer/` — RabbitMQ consumer wired to a usecase
- [ ] Unique index on `invoice_line(invoice_id, kind)` for `CANCELLATION` and `NOSHOW`
- [ ] DLQ + manual replay tool

## Acceptance criteria

- Replaying the same `reservation.cancelled.v1` twice creates exactly one line.
- A cancel within grace produces no line (engine returns empty for grace cancel).
