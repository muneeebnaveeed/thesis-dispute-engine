package dispute

import (
	"errors"
	"slices"
	"testing"
)

// run applies events in order and fails on the first rejection.
func run(t *testing.T, regime Regime, events ...Event) Dispute {
	t.Helper()
	d, err := New(regime)
	if err != nil {
		t.Fatalf("New(%s): %v", regime, err)
	}
	for _, e := range events {
		next, err := d.Apply(e)
		if err != nil {
			t.Fatalf("%s: Apply(%s) from %s: %v", regime, e, d.State, err)
		}
		d = next
	}
	return d
}

func TestHappyPaths(t *testing.T) {
	cases := []struct {
		name   string
		regime Regime
		events []Event
		want   State
	}{
		{
			name:   "SEPA DD: no-questions-asked refund straight from initiation",
			regime: RegimeEUSEPADirectDebit,
			events: []Event{EventIssueRefund, EventClose},
			want:   StateClosed,
		},
		{
			name:   "PSD2 card: investigate, fast refund, recover via chargeback, won",
			regime: RegimeEUPSD2Card,
			events: []Event{
				EventOpenInvestigation, EventSendQuestionnaire, EventReceiveQuestionnaire,
				EventIssueRefund, EventFileChargeback, EventAcknowledgeChargeback,
				EventSubmitEvidence, EventWinChargeback, EventClose,
			},
			want: StateClosed,
		},
		{
			name:   "Reg E: provisional credit, chargeback won, final credit",
			regime: RegimeUSRegE,
			events: []Event{
				EventOpenInvestigation, EventIssueRefund, EventFileChargeback,
				EventAcknowledgeChargeback, EventWinChargeback, EventIssueFinalCredit, EventClose,
			},
			want: StateClosed,
		},
		{
			name:   "Reg E: provisional credit, chargeback lost, credit reversed",
			regime: RegimeUSRegE,
			events: []Event{
				EventOpenInvestigation, EventIssueRefund, EventFileChargeback,
				EventAcknowledgeChargeback, EventSubmitEvidence, EventLoseChargeback,
				EventReverseProvisionalCredit, EventClose,
			},
			want: StateClosed,
		},
		{
			name:   "Reg Z: no credit path, chargeback filed from investigation",
			regime: RegimeUSRegZ,
			events: []Event{
				EventOpenInvestigation, EventFileChargeback, EventAcknowledgeChargeback,
				EventLoseChargeback, EventClose,
			},
			want: StateClosed,
		},
		{
			name:   "any regime: denied during investigation",
			regime: RegimeEUPSD2Card,
			events: []Event{EventOpenInvestigation, EventClose},
			want:   StateClosed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := run(t, tc.regime, tc.events...); got.State != tc.want {
				t.Errorf("final state = %s, want %s", got.State, tc.want)
			}
		})
	}
}

func TestRegimeGates(t *testing.T) {
	cases := []struct {
		name   string
		regime Regime
		setup  []Event
		event  Event
	}{
		{"SEPA DD never files a chargeback", RegimeEUSEPADirectDebit, []Event{EventIssueRefund}, EventFileChargeback},
		{"SEPA DD cannot be investigated then refunded via the card path", RegimeEUSEPADirectDebit, []Event{EventOpenInvestigation}, EventFileChargeback},
		{"PSD2 has no provisional credit to finalise", RegimeEUPSD2Card,
			[]Event{EventOpenInvestigation, EventIssueRefund, EventFileChargeback, EventAcknowledgeChargeback, EventWinChargeback}, EventIssueFinalCredit},
		{"PSD2 has no provisional credit to reverse", RegimeEUPSD2Card,
			[]Event{EventOpenInvestigation, EventIssueRefund, EventFileChargeback, EventAcknowledgeChargeback, EventLoseChargeback}, EventReverseProvisionalCredit},
		{"Reg E must investigate before crediting", RegimeUSRegE, nil, EventIssueRefund},
		{"Reg E files the chargeback after the credit, not instead of it", RegimeUSRegE, []Event{EventOpenInvestigation}, EventFileChargeback},
		{"Reg E cannot close a won chargeback without the final credit", RegimeUSRegE,
			[]Event{EventOpenInvestigation, EventIssueRefund, EventFileChargeback, EventAcknowledgeChargeback, EventWinChargeback}, EventClose},
		{"Reg Z has no refund step", RegimeUSRegZ, []Event{EventOpenInvestigation}, EventIssueRefund},
		{"nothing skips acknowledgement", RegimeUSRegE,
			[]Event{EventOpenInvestigation, EventIssueRefund, EventFileChargeback}, EventWinChargeback},
		{"closed disputes only move by appeal", RegimeEUPSD2Card, []Event{EventOpenInvestigation, EventClose}, EventOpenInvestigation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := run(t, tc.regime, tc.setup...)
			if _, err := d.Apply(tc.event); !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("Apply(%s) from %s: err = %v, want ErrInvalidTransition", tc.event, d.State, err)
			}
		})
	}
}

func TestAppeals(t *testing.T) {
	d := run(t, RegimeEUPSD2Card, EventOpenInvestigation, EventClose)

	if _, err := run(t, RegimeEUPSD2Card, EventOpenInvestigation).Apply(EventAppeal); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("appeal before closure: err = %v, want ErrInvalidTransition", err)
	}

	appealed, err := d.Apply(EventAppeal)
	if err != nil {
		t.Fatalf("first appeal: %v", err)
	}
	if appealed.State != StateInvestigating || appealed.Appeals != 1 {
		t.Errorf("after appeal = %+v, want INVESTIGATING with 1 appeal", appealed)
	}
	if d.Appeals != 0 {
		t.Error("Apply mutated its receiver")
	}

	closedAgain, err := appealed.Apply(EventClose)
	if err != nil {
		t.Fatalf("close after appeal: %v", err)
	}
	if _, err := closedAgain.Apply(EventAppeal); !errors.Is(err, ErrAppealsExhausted) {
		t.Errorf("second appeal: err = %v, want ErrAppealsExhausted", err)
	}
}

func TestAllowedMatchesApply(t *testing.T) {
	for _, regime := range AllRegimes() {
		for _, state := range reachable(t, regime) {
			d := Dispute{Regime: regime, State: state}
			allowed := d.Allowed()
			for _, e := range AllEvents() {
				_, err := d.Apply(e)
				if (err == nil) != slices.Contains(allowed, e) {
					t.Errorf("%s/%s: Allowed and Apply disagree on %s (err=%v)", regime, state, e, err)
				}
			}
		}
	}
}

// TestReachableStates pins the exact set of states each regime can visit. A
// change here is a change to the lifecycle and belongs in ADR 0002.
func TestReachableStates(t *testing.T) {
	want := map[Regime][]State{
		RegimeEUSEPADirectDebit: {
			StateInitiated, StateInvestigating, StateQuestionnaireSent, StateQuestionnaireReceived,
			StateSEPANQARefundIssued, StateClosed,
		},
		RegimeEUPSD2Card: {
			StateInitiated, StateInvestigating, StateQuestionnaireSent, StateQuestionnaireReceived,
			StateFastRefundIssued, StateChargebackFiled, StateChargebackAcknowledged,
			StateEvidenceSubmitted, StateChargebackWon, StateChargebackLost, StateClosed,
		},
		RegimeUSRegE: {
			StateInitiated, StateInvestigating, StateQuestionnaireSent, StateQuestionnaireReceived,
			StateProvisionalCreditIssued, StateChargebackFiled, StateChargebackAcknowledged,
			StateEvidenceSubmitted, StateChargebackWon, StateChargebackLost,
			StateFinalCreditIssued, StateProvisionalCreditReversed, StateClosed,
		},
		RegimeUSRegZ: {
			StateInitiated, StateInvestigating, StateQuestionnaireSent, StateQuestionnaireReceived,
			StateChargebackFiled, StateChargebackAcknowledged, StateEvidenceSubmitted,
			StateChargebackWon, StateChargebackLost, StateClosed,
		},
	}
	for _, regime := range AllRegimes() {
		got := reachable(t, regime)
		w := want[regime]
		slices.Sort(got)
		slices.Sort(w)
		if !slices.Equal(got, w) {
			t.Errorf("%s reachable states:\n got %v\nwant %v", regime, got, w)
		}
	}
}

func TestEveryReachableStateCanReachClosed(t *testing.T) {
	for _, regime := range AllRegimes() {
		for _, state := range reachable(t, regime) {
			if !canReach(regime, state, StateClosed) {
				t.Errorf("%s: %s cannot reach CLOSED (dead end)", regime, state)
			}
		}
	}
}

func TestRulesAreConsistent(t *testing.T) {
	for _, regime := range AllRegimes() {
		r, err := RulesFor(regime)
		if err != nil {
			t.Fatal(err)
		}
		if r.Regime != regime {
			t.Errorf("%s: Rules.Regime = %s", regime, r.Regime)
		}
		if r.HasRefundStep() != (r.RefundSLA > 0) {
			t.Errorf("%s: refund step %v but RefundSLA %v", regime, r.HasRefundStep(), r.RefundSLA)
		}
		if r.MerchantMayContest && !r.HasAdjudication {
			t.Errorf("%s: merchant may contest but there is no adjudication", regime)
		}
		if r.MaxAppeals < 1 {
			t.Errorf("%s: MaxAppeals = %d", regime, r.MaxAppeals)
		}
	}
	if _, err := RulesFor("MARS"); err == nil {
		t.Error("RulesFor(unknown): want error")
	}
	if _, err := New("MARS"); err == nil {
		t.Error("New(unknown): want error")
	}
}

// reachable is a breadth-first walk over the transition table from INITIATED,
// excluding appeals (which only ever lead back to INVESTIGATING).
func reachable(t *testing.T, regime Regime) []State {
	t.Helper()
	start, err := New(regime)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[State]bool{start.State: true}
	queue := []Dispute{start}
	for len(queue) > 0 {
		d := queue[0]
		queue = queue[1:]
		for _, e := range AllEvents() {
			if e == EventAppeal {
				continue
			}
			n, err := d.Apply(e)
			if err != nil || seen[n.State] {
				continue
			}
			seen[n.State] = true
			queue = append(queue, n)
		}
	}
	out := make([]State, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	return out
}

func canReach(regime Regime, from, to State) bool {
	seen := map[State]bool{from: true}
	queue := []State{from}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		if s == to {
			return true
		}
		for _, e := range AllEvents() {
			if e == EventAppeal {
				continue
			}
			n, err := (Dispute{Regime: regime, State: s}).Apply(e)
			if err != nil || seen[n.State] {
				continue
			}
			seen[n.State] = true
			queue = append(queue, n.State)
		}
	}
	return false
}
