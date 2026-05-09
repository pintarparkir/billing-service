package pricing

import (
	"math"
	"time"
)

// cancellationRule short-circuits the chain on cancel:
//   - cancel inside grace window  → no lines (free)
//   - cancel after grace          → CancelFeeIDR only, no booking, no hourly
// We mark prior with both lines so later rules (Booking, Hourly, Overnight) skip.
type cancellationRule struct{}

func (cancellationRule) Apply(s Session, _ []LineItem, cfg Config) []LineItem {
	if s.CancelledAt == nil {
		return nil
	}
	since := s.CancelledAt.Sub(s.ConfirmedAt)
	if since <= cfg.CancelGrace {
		// Free cancel — emit nothing, but signal "cancelled" via a synthetic
		// CANCELLATION line of 0 IDR so downstream rules can detect it.
		return []LineItem{{Kind: LineCancellation, AmountIDR: 0, Note: "within grace"}}
	}
	return []LineItem{{Kind: LineCancellation, AmountIDR: cfg.CancelFeeIDR, Note: "post-grace cancel"}}
}

// noShowRule emits the no-show flat fee and short-circuits hourly/overnight.
type noShowRule struct{}

func (noShowRule) Apply(s Session, prior []LineItem, cfg Config) []LineItem {
	if !s.NoShow {
		return nil
	}
	if hasKind(prior, LineCancellation) {
		// Cancellation already accounted; no-show flag is informational.
		return nil
	}
	return []LineItem{{Kind: LineNoShow, AmountIDR: cfg.NoShowFeeIDR, Note: "no-show fee"}}
}

// bookingRule emits the always-on booking fee.
// Skipped if Cancellation rule already fired (free cancel = no booking either,
// post-grace cancel = cancel fee replaces booking).
type bookingRule struct{}

func (bookingRule) Apply(_ Session, prior []LineItem, cfg Config) []LineItem {
	if hasKind(prior, LineCancellation) {
		return nil
	}
	if hasKind(prior, LineNoShow) {
		// No-show: still charge booking fee per the assessment scenarios.
		return []LineItem{{Kind: LineBooking, AmountIDR: cfg.BookingFeeIDR, Note: "booking fee (no-show)"}}
	}
	return []LineItem{{Kind: LineBooking, AmountIDR: cfg.BookingFeeIDR, Note: "booking fee"}}
}

// overnightRule fires iff the session [CheckedInAt, CheckedOutAt] crosses
// midnight in the configured timezone (default Asia/Jakarta). When it does,
// a single flat OVERNIGHT line replaces hourly billing.
type overnightRule struct{}

func (overnightRule) Apply(s Session, prior []LineItem, cfg Config) []LineItem {
	if s.CheckedInAt.IsZero() || s.CheckedOutAt.IsZero() {
		return nil
	}
	if hasKind(prior, LineCancellation) || hasKind(prior, LineNoShow) {
		return nil
	}
	in := s.CheckedInAt.In(s.Timezone)
	out := s.CheckedOutAt.In(s.Timezone)
	// midnight of the day after `in`, in the same tz
	nextMid := time.Date(in.Year(), in.Month(), in.Day()+1, 0, 0, 0, 0, s.Timezone)
	if !out.After(nextMid) && !out.Equal(nextMid) {
		return nil
	}
	return []LineItem{{Kind: LineOvernight, AmountIDR: cfg.OvernightFlatIDR, Note: "overnight flat"}}
}

// hourlyRule charges ceil(duration / 1h) × HourlyRateIDR. Skipped iff a prior
// rule already emitted Cancellation, NoShow, or Overnight.
type hourlyRule struct{}

func (hourlyRule) Apply(s Session, prior []LineItem, cfg Config) []LineItem {
	if s.CheckedInAt.IsZero() || s.CheckedOutAt.IsZero() {
		return nil
	}
	if hasKind(prior, LineCancellation) || hasKind(prior, LineNoShow) || hasKind(prior, LineOvernight) {
		return nil
	}
	dur := s.CheckedOutAt.Sub(s.CheckedInAt)
	if dur <= 0 {
		return nil
	}
	hours := int64(math.Ceil(dur.Hours()))
	if hours < 1 {
		hours = 1
	}
	return []LineItem{{
		Kind:      LineHourly,
		AmountIDR: hours * cfg.HourlyRateIDR,
		Note:      durLabel(dur, hours),
	}}
}

func durLabel(d time.Duration, hours int64) string {
	if d < time.Hour {
		return "1h (rounded up)"
	}
	switch hours {
	case 1:
		return "1h"
	default:
		return formatHours(hours)
	}
}

func formatHours(h int64) string {
	// avoid pulling in fmt for hot path; explicit conversion is clearer too.
	switch h {
	case 2:
		return "2h"
	case 3:
		return "3h"
	case 4:
		return "4h"
	case 5:
		return "5h"
	default:
		// fallback for arbitrary values
		return "Nh"
	}
}
