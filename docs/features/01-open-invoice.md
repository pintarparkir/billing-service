# Feature 01 — OpenInvoice

**Status:** ✅ shipped
**Owner:** billing-service

## Scope

Triggered by reservation-service when a reservation is created. Inserts an OPEN
invoice with a booking-fee line. Idempotent on `Idempotency-Key` (passed via
gRPC metadata).

## API contract

```proto
rpc OpenInvoice(OpenInvoiceRequest) returns (Invoice);

message OpenInvoiceRequest {
  string reservation_id = 1;
  string driver_id = 2;
}
```

## Algorithm

```
1. idempotency replay check (interceptor, scope=method, key from metadata)
2. BEGIN tx
3. INSERT invoice (status='OPEN', idempotency_key=<key>)
   ON CONFLICT (idempotency_key) DO NOTHING RETURNING ...
4. If not inserted (replay), SELECT existing → return.
5. INSERT invoice_line (kind='BOOKING', amount_idr=$BOOKING_FEE_IDR)
6. UPDATE invoice SET total_idr = $BOOKING_FEE_IDR
7. INSERT outbox_event 'billing.invoice.opened.v1'
8. COMMIT
9. cache idem response
```

## Tasks

- [ ] `repository.InvoiceRepository.Open`
- [ ] `usecase.OpenInvoice`
- [ ] gRPC handler + idempotency interceptor (mirror user-service)
- [ ] Outbox row in same tx
- [ ] Integration test for replay (same key → same id)

## Acceptance criteria

- Calling OpenInvoice twice with the same `Idempotency-Key` returns the same
  invoice id and creates exactly one row.
- Booking-fee line amount equals env `BOOKING_FEE_IDR` (default 5,000).
