// Package pricing computes invoice line-items from a reservation session.
// Pure functional: no I/O, no state, deterministic. The whole engine is
// testable from stdlib only.
//
// Inputs come in as a Session struct; outputs are a slice of LineItem.
// Composition: Engine holds a slice of Rule; Apply() runs each rule in order
// and concatenates non-nil emissions. Rules can short-circuit via PriorRules
// (e.g. OvernightRule replaces HourlyRule) by inspecting prior emissions.
package pricing

import "time"

// LineKind enumerates the billable line types we emit. Matches the DB enum
// `line_kind` in data/migrations/001_init.up.sql.
type LineKind string

const (
	LineBooking      LineKind = "BOOKING"
	LineHourly       LineKind = "HOURLY"
	LineOvernight    LineKind = "OVERNIGHT"
	LineCancellation LineKind = "CANCELLATION"
	LineNoShow       LineKind = "NOSHOW"
)

// VehicleType mirrors the reservation domain. Replicated here to keep this
// package zero-dependency on the rest of billing-service.
type VehicleType string

const (
	VehicleCar        VehicleType = "CAR"
	VehicleMotorcycle VehicleType = "MOTORCYCLE"
)

// Session is the authoritative input. The caller (CloseInvoice usecase or
// cancel-fee consumer) materialises it from reservation timestamps.
type Session struct {
	ConfirmedAt  time.Time
	CheckedInAt  time.Time
	CheckedOutAt time.Time
	CancelledAt  *time.Time // non-nil iff cancellation path
	NoShow       bool       // true iff worker marked the reservation EXPIRED
	VehicleType  VehicleType
	Timezone     *time.Location // default Asia/Jakarta if nil
}

// LineItem is one row destined for invoice_line.
type LineItem struct {
	Kind      LineKind
	AmountIDR int64
	Note      string
}

// Config carries the tariff knobs (default values match the assessment soal).
type Config struct {
	BookingFeeIDR    int64
	HourlyRateIDR    int64
	OvernightFlatIDR int64
	CancelFeeIDR     int64
	NoShowFeeIDR     int64
	CancelGrace      time.Duration
}

// DefaultConfig matches soal v1.0(1) tariffs.
func DefaultConfig() Config {
	return Config{
		BookingFeeIDR:    5000,
		HourlyRateIDR:    5000,
		OvernightFlatIDR: 20000,
		CancelFeeIDR:     5000,
		NoShowFeeIDR:     5000,
		CancelGrace:      2 * time.Minute,
	}
}
