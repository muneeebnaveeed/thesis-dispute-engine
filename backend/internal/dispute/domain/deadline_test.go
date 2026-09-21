package domain

import (
	"testing"
	"time"
)

func date(s string, loc *time.Location) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
	if err != nil {
		panic(err)
	}
	return t
}

func TestCalendarDueBusinessDaysSkipWeekendsAndHolidays(t *testing.T) {
	cal, err := NewCalendar("Europe/Budapest", []string{"2026-10-23"}) // Friday, a Hungarian national day
	if err != nil {
		t.Fatal(err)
	}
	loc := cal.Location
	cases := []struct {
		start string
		clock Clock
		want  string
	}{
		// PSD2 art. 73: a Friday-evening complaint is due by the end of Monday.
		{"2026-09-18 19:30", Clock{Kind: DeadlineRefund, Count: 1, Unit: BusinessDays}, "2026-09-21 23:59"},
		// A complaint on the day before a Friday holiday skips the holiday and the weekend.
		{"2026-10-22 09:00", Clock{Kind: DeadlineRefund, Count: 1, Unit: BusinessDays}, "2026-10-26 23:59"},
		// Ten business days from a Monday is the Monday two weeks on.
		{"2026-09-21 10:00", Clock{Kind: DeadlineRefund, Count: 10, Unit: BusinessDays}, "2026-10-05 23:59"},
		// Calendar days count weekends.
		{"2026-09-18 19:30", Clock{Kind: DeadlineResolution, Count: 45, Unit: CalendarDays}, "2026-11-02 23:59"},
	}
	for _, c := range cases {
		got := cal.Due(date(c.start, loc), c.clock)
		want := date(c.want, loc).Add(time.Minute - time.Nanosecond)
		if !got.Equal(want) {
			t.Errorf("Due(%s, %d %s) = %s, want %s", c.start, c.clock.Count, c.clock.Unit, got, want)
		}
	}
}

func TestCalendarDueUsesTheTenantZoneNotUTC(t *testing.T) {
	cal, _ := NewCalendar("America/New_York", nil)
	// 01:00 UTC on Saturday is still Friday evening in New York, so one business day ends Monday New York time.
	start := time.Date(2026, 9, 19, 1, 0, 0, 0, time.UTC)
	got := cal.Due(start, Clock{Count: 1, Unit: BusinessDays})
	if got.In(cal.Location).Weekday() != time.Monday || got.In(cal.Location).Day() != 21 {
		t.Errorf("due = %s, want end of Monday 21 September New York time", got.In(cal.Location))
	}
}

func TestNewCalendarRejectsBadInput(t *testing.T) {
	if _, err := NewCalendar("Mars/Olympus", nil); err == nil {
		t.Error("unknown zone accepted")
	}
	if _, err := NewCalendar("UTC", []string{"23/10/2026"}); err == nil {
		t.Error("malformed holiday accepted")
	}
}

func TestDeadlineStatus(t *testing.T) {
	due := time.Date(2026, 9, 21, 23, 59, 59, 0, time.UTC)
	before, after := due.Add(-time.Hour), due.Add(time.Hour)
	d := Deadline{DueAt: due}
	if got := d.Status(before); got != DeadlineRunning {
		t.Errorf("open before due: %s", got)
	}
	if got := d.Status(after); got != DeadlineBreached {
		t.Errorf("open after due: %s", got)
	}
	d.MetAt = &before
	if got := d.Status(after); got != DeadlineMet {
		t.Errorf("met before due: %s", got)
	}
	d.MetAt = &after
	if got := d.Status(after); got != DeadlineLate {
		t.Errorf("met after due: %s", got)
	}
	d.MetAt = nil
	d.VoidedAt = &before
	if got := d.Status(after); got != DeadlineVoid {
		t.Errorf("voided: %s", got)
	}
}

func TestOpeningDeadlinesPerRegime(t *testing.T) {
	cal := DefaultCalendar()
	opened := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC) // a Monday
	want := map[Regime]map[DeadlineKind]string{
		RegimeEUPSD2Card:        {DeadlineRefund: "2026-09-22", DeadlineResolution: "2026-10-12"},
		RegimeEUSEPADirectDebit: {DeadlineRefund: "2026-10-05", DeadlineResolution: "2026-10-12"},
		RegimeUSRegE:            {DeadlineRefund: "2026-10-05", DeadlineResolution: "2026-11-05"},
		RegimeUSRegZ:            {DeadlineAcknowledge: "2026-10-21", DeadlineResolution: "2026-12-20"},
	}
	for regime, kinds := range want {
		r, _ := RulesFor(regime)
		got := r.OpeningDeadlines(opened, cal)
		if len(got) != len(kinds) {
			t.Fatalf("%s: %d deadlines, want %d", regime, len(got), len(kinds))
		}
		for _, d := range got {
			if day := d.DueAt.UTC().Format("2006-01-02"); day != kinds[d.Kind] {
				t.Errorf("%s %s due %s, want %s", regime, d.Kind, day, kinds[d.Kind])
			}
			if d.Basis == "" || d.Cycle != 0 || !d.StartedAt.Equal(opened) {
				t.Errorf("%s %s: %+v", regime, d.Kind, d)
			}
		}
	}
}

func TestAppealRestartsOnlyTheResolutionClock(t *testing.T) {
	r, _ := RulesFor(RegimeUSRegE)
	got := r.AppealDeadlines(1, time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC), DefaultCalendar())
	if len(got) != 1 || got[0].Kind != DeadlineResolution || got[0].Cycle != 1 {
		t.Fatalf("appeal deadlines = %+v", got)
	}
}

func TestSettle(t *testing.T) {
	cases := []struct {
		kind  DeadlineKind
		state State
		want  Settlement
	}{
		{DeadlineRefund, StateFastRefundIssued, Met},
		{DeadlineRefund, StateProvisionalCreditIssued, Met},
		{DeadlineRefund, StateFinalCreditIssued, Met},
		{DeadlineRefund, StateClosed, Void},
		{DeadlineRefund, StateInvestigating, Untouched},
		{DeadlineAcknowledge, StateInvestigating, Untouched},
		{DeadlineAcknowledge, StateClosed, Void},
		{DeadlineResolution, StateChargebackWon, Untouched},
		{DeadlineResolution, StateClosed, Met},
	}
	for _, c := range cases {
		if got := Settle(c.kind, c.state); got != c.want {
			t.Errorf("Settle(%s, %s) = %v, want %v", c.kind, c.state, got, c.want)
		}
	}
}
