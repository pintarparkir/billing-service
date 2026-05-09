package pricing

import "time"

// Rule is the unit of pricing logic. Implementations inspect the Session and
// the lines previously emitted by earlier rules; they return any new lines or
// nil to skip. Order in Engine.rules is significant — see NewDefaultEngine.
type Rule interface {
	Apply(s Session, prior []LineItem, cfg Config) []LineItem
}

// Engine runs rules in order and concatenates their emissions.
type Engine struct {
	cfg   Config
	rules []Rule
}

// NewDefaultEngine wires the canonical rule set in the order they must execute.
//
//   1. Cancellation — if cancelled, all other rules are short-circuited to
//      0/booking-or-cancel-fee depending on grace window.
//   2. NoShow — if the worker flagged the row, emit a flat fee, no hourly.
//   3. Booking — always charge unless cancelled within grace.
//   4. Overnight — if session crosses 00:00 WIB, flat replaces hourly.
//   5. Hourly — ceil(duration/hour) × rate, skipped iff Overnight emitted.
//
// All rules are pure: they never call out, never read clock state, never log.
func NewDefaultEngine(cfg Config) *Engine {
	return &Engine{
		cfg: cfg,
		rules: []Rule{
			cancellationRule{},
			noShowRule{},
			bookingRule{},
			overnightRule{},
			hourlyRule{},
		},
	}
}

// Apply runs the rule chain. Returned slice is safe to mutate by the caller.
func (e *Engine) Apply(s Session) []LineItem {
	if s.Timezone == nil {
		s.Timezone, _ = time.LoadLocation("Asia/Jakarta")
	}
	var out []LineItem
	for _, r := range e.rules {
		out = append(out, r.Apply(s, out, e.cfg)...)
	}
	return out
}

// hasKind reports whether `prior` already contains a line of the given kind.
// Used by later rules to short-circuit (e.g. Hourly skips when Overnight ran).
func hasKind(prior []LineItem, k LineKind) bool {
	for _, li := range prior {
		if li.Kind == k {
			return true
		}
	}
	return false
}
