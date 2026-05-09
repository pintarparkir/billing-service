# Feature 02 — CloseInvoice

**Status:** ⏳ in progress (gRPC handler pending buf generate)
**Owner:** billing-service

## Scope

Triggered on `reservation.checked_out.v1` (consumer) or directly via gRPC by
reservation-service. Applies the pricing engine to compute charges, appends the
relevant lines, sets `status=CLOSED`, emits `billing.invoice.closed.v1`.

## Algorithm

```
1. SELECT invoice + lines
2. If status != OPEN → no-op (idempotent on already-closed)
3. Build pricing.Session from check-in/out timestamps + vehicle type
4. lines := pricing.Apply(session)   ← pure function, no I/O
5. BEGIN tx
6. INSERT each new line
7. UPDATE invoice SET status='CLOSED', total_idr=SUM(lines), closed_at=now()
8. INSERT outbox_event 'billing.invoice.closed.v1'
9. COMMIT
```

## Tasks

- [ ] `usecase.CloseInvoice` orchestration
- [ ] Wire `pkg/pricing.NewDefaultEngine(cfg)` from env tariffs
- [ ] Idempotent on already-closed invoice (returns existing without re-applying)
- [ ] Outbox row in same tx
- [ ] Integration test: open → close → re-close (no double-billing)

## Acceptance criteria

- Closing an already-CLOSED invoice returns the same total without inserting new lines.
- Closing an OPEN invoice with a 30-min session + 1 night crossing returns
  booking 5k + overnight 20k = 25k (assuming default tariffs).
