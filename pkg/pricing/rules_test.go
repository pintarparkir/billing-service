package pricing

import (
	"testing"
	"time"
)

// jakarta is the canonical pricing timezone used in tests.
var jakarta = mustLoc("Asia/Jakarta")

func mustLoc(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// total walks the engine's emissions and sums the IDR amount.
func total(lines []LineItem) int64 {
	var sum int64
	for _, l := range lines {
		sum += l.AmountIDR
	}
	return sum
}

// kindAmount returns the first line of the given kind, or 0 if absent.
func kindAmount(lines []LineItem, k LineKind) int64 {
	for _, l := range lines {
		if l.Kind == k {
			return l.AmountIDR
		}
	}
	return 0
}

func TestPricing_HappyPath_30Min(t *testing.T) {
	// 30 min session → ceil(0.5h) = 1h hourly + booking = 5k + 5k = 10k
	in := time.Date(2026, 5, 9, 10, 0, 0, 0, jakarta)
	out := in.Add(30 * time.Minute)
	got := NewDefaultEngine(DefaultConfig()).Apply(Session{
		ConfirmedAt: in.Add(-1 * time.Minute), CheckedInAt: in, CheckedOutAt: out,
		VehicleType: VehicleCar, Timezone: jakarta,
	})
	if total(got) != 10000 {
		t.Fatalf("want 10000, got %d (lines=%+v)", total(got), got)
	}
}

func TestPricing_3Hours5Min_StartedHourRounding(t *testing.T) {
	// 3h05m → ceil = 4h hourly + booking = 5k + 4×5k = 25k
	in := time.Date(2026, 5, 9, 10, 0, 0, 0, jakarta)
	out := in.Add(3*time.Hour + 5*time.Minute)
	got := NewDefaultEngine(DefaultConfig()).Apply(Session{
		ConfirmedAt: in, CheckedInAt: in, CheckedOutAt: out, VehicleType: VehicleCar, Timezone: jakarta,
	})
	if total(got) != 25000 {
		t.Fatalf("want 25000, got %d (lines=%+v)", total(got), got)
	}
}

func TestPricing_OvernightCrossesMidnight(t *testing.T) {
	// In: 2026-05-09 22:30 WIB, out: 2026-05-10 06:00 WIB
	// Crosses midnight → overnight flat 20k + booking 5k = 25k. No hourly.
	in := time.Date(2026, 5, 9, 22, 30, 0, 0, jakarta)
	out := time.Date(2026, 5, 10, 6, 0, 0, 0, jakarta)
	got := NewDefaultEngine(DefaultConfig()).Apply(Session{
		ConfirmedAt: in, CheckedInAt: in, CheckedOutAt: out, VehicleType: VehicleCar, Timezone: jakarta,
	})
	if total(got) != 25000 {
		t.Fatalf("want 25000, got %d (lines=%+v)", total(got), got)
	}
	if kindAmount(got, LineHourly) != 0 {
		t.Errorf("expected hourly to be skipped when overnight fires; got %+v", got)
	}
	if kindAmount(got, LineOvernight) != 20000 {
		t.Errorf("expected overnight 20000; got %+v", got)
	}
}

func TestPricing_CancelWithinGrace_FreeNoBookingFee(t *testing.T) {
	// Cancelled 1 min after confirm → grace window → 0 IDR
	confirm := time.Date(2026, 5, 9, 10, 0, 0, 0, jakarta)
	cancel := confirm.Add(1 * time.Minute)
	got := NewDefaultEngine(DefaultConfig()).Apply(Session{
		ConfirmedAt: confirm, CancelledAt: &cancel, VehicleType: VehicleCar, Timezone: jakarta,
	})
	if total(got) != 0 {
		t.Fatalf("want 0, got %d (lines=%+v)", total(got), got)
	}
	if kindAmount(got, LineBooking) != 0 {
		t.Errorf("booking fee must NOT apply on grace cancel; got %+v", got)
	}
}

func TestPricing_CancelAfterGrace_5kOnly(t *testing.T) {
	// Cancelled 10 min after confirm → cancel fee 5k, no booking, no hourly
	confirm := time.Date(2026, 5, 9, 10, 0, 0, 0, jakarta)
	cancel := confirm.Add(10 * time.Minute)
	got := NewDefaultEngine(DefaultConfig()).Apply(Session{
		ConfirmedAt: confirm, CancelledAt: &cancel, VehicleType: VehicleCar, Timezone: jakarta,
	})
	if total(got) != 5000 {
		t.Fatalf("want 5000, got %d (lines=%+v)", total(got), got)
	}
	if kindAmount(got, LineBooking) != 0 {
		t.Errorf("booking fee must NOT stack with post-grace cancel; got %+v", got)
	}
}

func TestPricing_NoShow_5k(t *testing.T) {
	// No check-in within hold window → NoShow flag set by worker
	// Per assessment: noshow fee 5k + booking fee 5k = 10k total
	confirm := time.Date(2026, 5, 9, 10, 0, 0, 0, jakarta)
	got := NewDefaultEngine(DefaultConfig()).Apply(Session{
		ConfirmedAt: confirm, NoShow: true, VehicleType: VehicleCar, Timezone: jakarta,
	})
	if kindAmount(got, LineNoShow) != 5000 {
		t.Errorf("expected noshow 5000; got %+v", got)
	}
	if kindAmount(got, LineHourly) != 0 {
		t.Errorf("hourly must be skipped on no-show; got %+v", got)
	}
}

func TestPricing_OneSecondPastHour_RoundsUp(t *testing.T) {
	// 1h 1s session → ceil(1.0003h) = 2h hourly + booking = 5k + 10k = 15k
	in := time.Date(2026, 5, 9, 10, 0, 0, 0, jakarta)
	out := in.Add(time.Hour + time.Second)
	got := NewDefaultEngine(DefaultConfig()).Apply(Session{
		ConfirmedAt: in, CheckedInAt: in, CheckedOutAt: out, VehicleType: VehicleCar, Timezone: jakarta,
	})
	if total(got) != 15000 {
		t.Fatalf("want 15000, got %d (lines=%+v)", total(got), got)
	}
}

func TestPricing_ExactMidnightCrossing(t *testing.T) {
	// In: 23:00, out: 00:00 next-day exactly → considered overnight
	in := time.Date(2026, 5, 9, 23, 0, 0, 0, jakarta)
	out := time.Date(2026, 5, 10, 0, 0, 0, 0, jakarta)
	got := NewDefaultEngine(DefaultConfig()).Apply(Session{
		ConfirmedAt: in, CheckedInAt: in, CheckedOutAt: out, VehicleType: VehicleCar, Timezone: jakarta,
	})
	if kindAmount(got, LineOvernight) != 20000 {
		t.Errorf("expected overnight at exact midnight crossing; got %+v", got)
	}
}

func TestPricing_TableDriven_EdgeCases(t *testing.T) {
	cfg := DefaultConfig()
	in := time.Date(2026, 5, 9, 10, 0, 0, 0, jakarta)

	cases := []struct {
		name string
		s    Session
		want int64
	}{
		{
			name: "1 hour exact",
			s:    Session{ConfirmedAt: in, CheckedInAt: in, CheckedOutAt: in.Add(time.Hour), VehicleType: VehicleCar, Timezone: jakarta},
			want: 10000, // booking 5k + 1h 5k
		},
		{
			name: "59 minutes",
			s:    Session{ConfirmedAt: in, CheckedInAt: in, CheckedOutAt: in.Add(59 * time.Minute), VehicleType: VehicleCar, Timezone: jakarta},
			want: 10000, // booking 5k + ceil(59m)=1h 5k
		},
		{
			name: "5 hours flat",
			s:    Session{ConfirmedAt: in, CheckedInAt: in, CheckedOutAt: in.Add(5 * time.Hour), VehicleType: VehicleCar, Timezone: jakarta},
			want: 30000, // booking 5k + 5×5k
		},
	}
	eng := NewDefaultEngine(cfg)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := eng.Apply(tc.s)
			if total(got) != tc.want {
				t.Errorf("want %d, got %d (lines=%+v)", tc.want, total(got), got)
			}
		})
	}
}

func TestPricing_OverstayBilledAtNormalRate(t *testing.T) {
	// Driver checked in at 10:00, hold expired at 11:00, but stayed until 13:30.
	// Overstay is billed at the same hourly rate; no penalty in MVP.
	// 3h30m → ceil = 4h. Booking 5k + 4×5k = 25k.
	in := time.Date(2026, 5, 9, 10, 0, 0, 0, jakarta)
	out := in.Add(3*time.Hour + 30*time.Minute)
	got := NewDefaultEngine(DefaultConfig()).Apply(Session{
		ConfirmedAt: in, CheckedInAt: in, CheckedOutAt: out, VehicleType: VehicleCar, Timezone: jakarta,
	})
	if total(got) != 25000 {
		t.Fatalf("want 25000, got %d (lines=%+v)", total(got), got)
	}
}
