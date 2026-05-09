# Feature 05 — Outbox publisher

**Status:** ✅ shipped
**Owner:** billing-service

Same pattern as `reservation-service/docs/features/07-outbox-publisher.md`.
Publishes `billing.invoice.opened.v1` and `billing.invoice.closed.v1`.

## Tasks

- [ ] Re-use the outbox publisher pattern from reservation-service
- [ ] Metric: `billing_outbox_published_total{event_type=...}`

## Acceptance criteria

Identical to reservation-service's outbox-publisher acceptance criteria.
