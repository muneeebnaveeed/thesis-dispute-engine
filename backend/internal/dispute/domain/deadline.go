package domain

import (
	"fmt"
	"time"
)

// DeadlineKind names a regulatory clock; each regime starts a subset of them when a dispute opens.
type DeadlineKind string

// Deadline kinds.
const (
	// DeadlineRefund: the customer is made whole (provisional credit, fast refund or no-questions-asked refund).
	DeadlineRefund DeadlineKind = "REFUND"
	// DeadlineAcknowledge: the dispute is acknowledged, which this engine equates with opening the investigation.
	DeadlineAcknowledge DeadlineKind = "ACKNOWLEDGE"
	// DeadlineResolution: the outcome is determined and the dispute closed.
	DeadlineResolution DeadlineKind = "RESOLUTION"
)

// DayUnit says how a clock counts days; regulations mix the two.
type DayUnit string

// Day units.
const (
	BusinessDays DayUnit = "BUSINESS_DAYS"
	CalendarDays DayUnit = "CALENDAR_DAYS"
)

// Clock is one regulatory deadline as a regime states it; Basis is the citation shown to analysts.
type Clock struct {
	Kind  DeadlineKind
	Count int
	Unit  DayUnit
	Basis string
}

// DeadlineStatus is where a clock stands at a moment.
type DeadlineStatus string

// Deadline statuses.
const (
	DeadlineRunning  DeadlineStatus = "RUNNING"
	DeadlineMet      DeadlineStatus = "MET"
	DeadlineLate     DeadlineStatus = "LATE"     // satisfied, but after the due time
	DeadlineBreached DeadlineStatus = "BREACHED" // due time passed, still open
	DeadlineVoid     DeadlineStatus = "VOID"     // no longer applies: the dispute ended another way
)

// Deadline is one running or settled clock on a dispute. Cycle is 0 for the opening clocks and the appeal
// number for the resolution clock an appeal restarts.
type Deadline struct {
	Kind      DeadlineKind
	Cycle     int
	StartedAt time.Time
	DueAt     time.Time
	MetAt     *time.Time
	VoidedAt  *time.Time
	Basis     string
}

// Status evaluates the clock at now.
func (d Deadline) Status(now time.Time) DeadlineStatus {
	switch {
	case d.MetAt != nil && !d.MetAt.After(d.DueAt):
		return DeadlineMet
	case d.MetAt != nil:
		return DeadlineLate
	case d.VoidedAt != nil:
		return DeadlineVoid
	case now.After(d.DueAt):
		return DeadlineBreached
	default:
		return DeadlineRunning
	}
}

// Open reports whether the clock is still counting.
func (d Deadline) Open() bool { return d.MetAt == nil && d.VoidedAt == nil }

// Calendar turns a clock into a due time: deadlines fall at the end of the day in the tenant's zone, and
// business-day clocks skip weekends and the tenant's holidays.
type Calendar struct {
	Location *time.Location
	Holidays map[string]struct{} // "2006-01-02" in Location
}

// DefaultCalendar is UTC with weekends only.
func DefaultCalendar() Calendar { return Calendar{Location: time.UTC} }

// NewCalendar builds a calendar; an unknown zone name is an error so a misconfigured tenant is noticed, not silently UTC.
func NewCalendar(zone string, holidays []string) (Calendar, error) {
	loc := time.UTC
	if zone != "" {
		var err error
		if loc, err = time.LoadLocation(zone); err != nil {
			return Calendar{}, fmt.Errorf("domain: calendar zone %q: %w", zone, err)
		}
	}
	c := Calendar{Location: loc, Holidays: make(map[string]struct{}, len(holidays))}
	for _, h := range holidays {
		if _, err := time.ParseInLocation("2006-01-02", h, loc); err != nil {
			return Calendar{}, fmt.Errorf("domain: calendar holiday %q: %w", h, err)
		}
		c.Holidays[h] = struct{}{}
	}
	return c, nil
}

// IsBusinessDay reports whether t falls on a weekday that is not a holiday.
func (c Calendar) IsBusinessDay(t time.Time) bool {
	t = t.In(c.loc())
	if wd := t.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false
	}
	_, holiday := c.Holidays[t.Format("2006-01-02")]
	return !holiday
}

// Due returns when a clock started at start runs out: the last instant of the Nth day counted from the day after
// start, so "within 1 business day" of a Friday evening is the end of Monday.
func (c Calendar) Due(start time.Time, clock Clock) time.Time {
	loc := c.loc()
	day := start.In(loc)
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	for n := 0; n < clock.Count; {
		day = day.AddDate(0, 0, 1)
		if clock.Unit == CalendarDays || c.IsBusinessDay(day) {
			n++
		}
	}
	return day.AddDate(0, 0, 1).Add(-time.Nanosecond)
}

func (c Calendar) loc() *time.Location {
	if c.Location == nil {
		return time.UTC
	}
	return c.Location
}

// OpeningDeadlines are the clocks a regime starts when a dispute opens.
func (r Rules) OpeningDeadlines(openedAt time.Time, cal Calendar) []Deadline {
	out := make([]Deadline, 0, len(r.Clocks))
	for _, c := range r.Clocks {
		out = append(out, Deadline{Kind: c.Kind, Cycle: 0, StartedAt: openedAt, DueAt: cal.Due(openedAt, c), Basis: c.Basis})
	}
	return out
}

// AppealDeadlines are the clocks an appeal restarts: the resolution clock only, numbered by the appeal.
func (r Rules) AppealDeadlines(appeal int, at time.Time, cal Calendar) []Deadline {
	var out []Deadline
	for _, c := range r.Clocks {
		if c.Kind == DeadlineResolution {
			out = append(out, Deadline{Kind: c.Kind, Cycle: appeal, StartedAt: at, DueAt: cal.Due(at, c), Basis: c.Basis})
		}
	}
	return out
}

// Settlement is what entering a state does to an open clock: nothing, meets it, or voids it.
type Settlement int

// Settlements.
const (
	Untouched Settlement = iota
	Met
	Void
)

// Settle says what entering state does to an open clock of a kind. A refund clock is met by any credit and void when
// the dispute closes without one; the acknowledgement clock is met by opening the investigation; the resolution
// clock is met only by closing.
func Settle(kind DeadlineKind, entered State) Settlement {
	switch kind {
	case DeadlineRefund:
		switch entered {
		case StateProvisionalCreditIssued, StateFastRefundIssued, StateSEPANQARefundIssued, StateFinalCreditIssued:
			return Met
		case StateClosed:
			return Void
		}
	case DeadlineAcknowledge:
		if entered != StateInitiated {
			return Met
		}
	case DeadlineResolution:
		if entered == StateClosed {
			return Met
		}
	}
	return Untouched
}

func (r Rules) hasClock(kind DeadlineKind) bool {
	for _, c := range r.Clocks {
		if c.Kind == kind {
			return true
		}
	}
	return false
}
