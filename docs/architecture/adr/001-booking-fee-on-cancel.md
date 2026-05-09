# ADR-005: Cancellation fee structure & no-show fee (soal v1.0(1) adjustment)

**Status:** Accepted (revised 2026-05-02 for soal v1.0(1))
**Date:** 2026-04-27 · **Revised:** 2026-05-02

## Context

The original soal (v1.0) carried two competing fee figures for no-show:

- *Reservation hold time bullet:* "the reservation … will be cost 5,000 IDR"
- *Cancellation policy bullet:* "no-show … fee is 10,000 IDR"

The cancellation policy bullet also defined three cancel windows (0 / 5,000 / 10,000 IDR).

The **revised soal v1.0(1) removed the entire "Cancellation policy" bullet**. What remains:

- Booking fee: 5,000 IDR on confirm (still explicit)
- No-show: 5,000 IDR (only figure left, in the hold-time bullet)
- Cancellation fee structure: **not specified by soal anymore**

But the testing requirements in v1.0(1) still mandate a **"cancellation policy"** scenario and a **"wrong-spot penalty"** scenario.

## Decision

We have to keep cancellation policy (test requires it), but the specific fee structure becomes our assumption:

1. **Cancel ≤ 2 min after CONFIRMED** → fee **0 IDR** (soft-cancel grace window)
2. **Cancel > 2 min, before check-in** → fee **5,000 IDR**
3. **No-show after 1 h** → fee **5,000 IDR** (per soal v1.0(1) hold-time bullet) + state EXPIRED
4. **Booking fee on void:** within the 2-min grace, the booking fee line item is *not* added to the invoice (invoice voided). After grace, it stays. This avoids refund operations.

All amounts are **configurable via env** (`CANCEL_LATE_FEE_IDR`, `NOSHOW_FEE_IDR`) so business can tune without redeploy.

## Why this specific structure

- **Anchor on what soal still gives us.** 5,000 IDR is the only no-show figure left in v1.0(1) — using anything else would contradict explicit soal text.
- **Symmetry between cancel-late and no-show.** Both are "you reserved but didn't park" — same harm to inventory, same fee feels fair.
- **Grace window of 2 min** matches what the original (v1.0) specified, before the bullet was removed. Keeping the duration preserves UX continuity for any user-facing copy already drafted against the v1.0 spec.
- **Clean accounting.** Voiding the invoice within grace avoids PG refund fees and dunning operations.

## Implementation

- `pkg/configs/type.go` — `NoShowFeeIDR` default `5_000`, `CancelLateFeeIDR` default `5_000`, `CancelGraceSeconds` default `120`
- `pkg/pricing/pricing.go` — `DefaultConfig()` reflects same defaults
- `pkg/pricing/rules.go::cancellationRule` — emits NOSHOW or CANCELLATION line item
- `internal/billing/usecase/handle_event.go::HandleNoShow` — adds 5,000 IDR `LINE_NOSHOW`
- `internal/reservation/usecase/cancel_reservation.go` — computes cancel fee from `cancelLateIDR`/`cancelGrace` config
- Tests: `pkg/pricing/pricing_test.go::TestPricing_NoShow_5k`, `TestPricing_Cancel*`

## Consequences

- **+** Solution stays compliant with soal v1.0(1) — only uses figures explicitly stated.
- **+** Cancellation policy test scenario still passes with sensible behaviour.
- **+** Tariffs swappable via env if business wants to introduce 10,000 / 15,000 etc later.
- **−** Cancellation fee structure is now an explicit assumption — surfaced in README §2.1 so reviewer sees it called out.

## Alternatives considered

- **Charge no-show 10,000 IDR (per old v1.0)** — rejected: contradicts the only no-show figure remaining in v1.0(1).
- **No cancel fee at all (since soal removed bullet)** — rejected: testing requirements still list "cancellation policy" as a mandatory scenario, which would be empty/trivial without a real fee.
- **Charge booking fee + cancel fee additively** — rejected: confusing for customer (10k total for a cancel feels punitive when soal doesn't mandate it).
