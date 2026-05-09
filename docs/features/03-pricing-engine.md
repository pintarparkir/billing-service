# Feature 03 — Pricing engine

**Status:** ✅ shipped
**Owner:** billing-service

## Scope

Pure-functional pricing rules. No I/O. The engine takes a `Session` (timestamps,
flags) and returns a slice of `LineItem`. Easy to test, easy to A/B.

## Public API

```go
type Session struct {
    CheckedInAt   time.Time
    CheckedOutAt  time.Time
    ConfirmedAt   time.Time
    CancelledAt   *time.Time
    NoShow        bool
    VehicleType   VehicleType
    Timezone      *time.Location  // default Asia/Jakarta
}

type LineItem struct { Kind LineKind; AmountIDR int64; Note string }

type Engine struct { rules []Rule }
func NewDefaultEngine(cfg Config) *Engine
func (e *Engine) Apply(s Session) []LineItem
```

## Rules (default config)

| Rule              | Logic                                                          |
|-------------------|----------------------------------------------------------------|
| `BookingFeeRule`  | Always emits 5,000 IDR (skipped if soft-cancelled in grace)    |
| `HourlyRule`      | `ceil(duration_min / 60) * 5,000`. Skipped if Overnight fires. |
| `OvernightRule`   | If session crosses 00:00 WIB → flat 20,000 IDR (replaces hourly)|
| `CancellationRule`| 0 IDR within 2-min grace, 5,000 IDR after                      |
| `NoShowRule`      | 5,000 IDR when `s.NoShow == true`                              |

See ADR-001 in this service for the cancel-fee rationale (carried over from
former ADR-005 in the parent repo).

## Test plan

```go
TestPricing_HappyPath_30Min                  // booking 5k + 1h 5k = 10k
TestPricing_3Hours5Min_StartedHourRounding   // 5k + 4×5k = 25k
TestPricing_OvernightCrossesMidnight         // booking 5k + flat 20k = 25k
TestPricing_CancelWithinGrace_FreeNoBookingFee   // 0
TestPricing_CancelAfterGrace_5kOnly          // 5k
TestPricing_NoShow_5k                        // 5k
TestPricing_OverstayBilledAtNormalRate       // billed at standard rate, no penalty
TestPricing_TableDriven_EdgeCases            // 1-second-past-hour rounds up; midnight exact
```

## Tasks

- [ ] `pkg/pricing/engine.go` + `rules.go`
- [ ] `pkg/pricing/config.go` reads tariff env vars
- [ ] All eight `TestPricing_*` tests pass
- [ ] Doc table of "input → expected output" lives next to the rules file

## Acceptance criteria

- Engine has zero imports outside stdlib (`time`, `math`).
- Adding a new rule (e.g. EV surcharge) is one file; existing rules unchanged.
- All scenarios from the table above pass.
