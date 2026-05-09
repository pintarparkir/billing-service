# Demo Walkthrough — billing-service

billing-service has no public REST surface. It's exercised in two ways:
1. **`go test ./pkg/pricing/...`** — runs the pricing engine through 13 cases
   without any infrastructure.
2. **End-to-end via reservation-service** — start both services and watch the
   billing rows pop into Postgres as RabbitMQ events flow.

## Setup

```bash
cd ../infra && docker compose up -d
cd ../billing-service
cp configs/.env.example configs/.env
make migrate-up
make run
```

`make run` starts:
- The outbox publisher worker (1 s tick).
- The RabbitMQ consumer bound to `reservation.cancelled.v1`,
  `reservation.expired.v1`, `reservation.checked_out.v1`.

## Scenario 1 — Pricing engine (offline, no infra needed)

```bash
go test -v ./pkg/pricing/...
```

You'll see:
- `TestPricing_HappyPath_30Min`               — booking 5k + 1h 5k = **10,000**
- `TestPricing_3Hours5Min_StartedHourRounding` — 5k + 4×5k = **25,000**
- `TestPricing_OvernightCrossesMidnight`      — booking 5k + flat 20k = **25,000**
- `TestPricing_CancelWithinGrace_FreeNoBookingFee` — **0**
- `TestPricing_CancelAfterGrace_5kOnly`       — **5,000**
- `TestPricing_NoShow_5k`                     — noshow 5k + booking 5k = **10,000**
- `TestPricing_OneSecondPastHour_RoundsUp`    — 5k + 2×5k = **15,000**
- `TestPricing_ExactMidnightCrossing`         — overnight engages
- `TestPricing_OverstayBilledAtNormalRate`    — 4h × 5k + booking = **25,000**
- `TestPricing_TableDriven_EdgeCases`         — 1h, 59m, 5h scenarios

**Talking points:**
- Engine has zero imports outside stdlib (`math`, `time`).
- Adding a new rule (EV surcharge, membership discount) = new file in
  `pkg/pricing/rules.go`; existing rules unchanged.
- Same engine handles cancel-fee, no-show-fee, and full close — the consumer
  just supplies a different `Session` shape.

## Scenario 2 — End-to-end via reservation-service

```bash
# Terminal 1: infra
cd ../infra && docker compose up -d

# Terminal 2: reservation-service
cd ../reservation-service && make run

# Terminal 3: billing-service
cd ../billing-service && make run

# Terminal 4: drive a reservation
DRIVER_TOKEN="eyJhbGciOiJSUzI1NiJ9.$(printf '{"sub":"demo-001","phone":"+628111","exp":9999999999}' | base64).x"
RID=$(curl -s -X POST http://localhost:8081/v1/reservations \
  -H "Authorization: Bearer $DRIVER_TOKEN" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"vehicle_type":"CAR","mode":"SYSTEM_ASSIGNED"}' | jq -r .id)
curl -s -X POST "http://localhost:8081/v1/reservations/$RID/confirm" -H "Authorization: Bearer $DRIVER_TOKEN" >/dev/null
curl -s -X POST "http://localhost:8081/v1/reservations/$RID/check-in" -H "Authorization: Bearer $DRIVER_TOKEN" \
  -d '{"latitude":-6.2088,"longitude":106.8456}' >/dev/null
sleep 5
curl -s -X POST "http://localhost:8081/v1/reservations/$RID/check-out" -H "Authorization: Bearer $DRIVER_TOKEN" >/dev/null

# Watch billing's logs — you should see CloseInvoice run on
# reservation.checked_out.v1, then billing.invoice.closed.v1 published.

# Inspect the resulting rows
docker exec -it parkir-postgres psql -U postgres -d billing_service -c \
  "SELECT i.id, i.status, i.total_idr, l.kind, l.amount_idr
     FROM invoice i
LEFT JOIN invoice_line l ON l.invoice_id = i.id
ORDER BY i.created_at DESC, l.created_at LIMIT 20;"
```

Expected: one invoice with `status=CLOSED`, `total_idr=10000`, lines
`BOOKING 5000` + `HOURLY 5000`.

## Scenario 3 — Cancel-fee idempotency

Replay the same `reservation.cancelled.v1` event twice (e.g. requeue from the
RabbitMQ management UI). Only one CANCELLATION line is added — guarded by the
`uq_line_one_off` partial unique index on `(invoice_id, kind)`.

## What's deferred

- Real gRPC server (`OpenInvoice`/`CloseInvoice`/`GetInvoice` direct from
  reservation-service) — the proto contract is in
  `api/proto/billing/v1/billing.proto`; handlers land once `buf generate` runs.
  Until then, all flows reach billing through the RabbitMQ consumer path,
  which is itself event-driven and exercised by the e2e scenarios above.
