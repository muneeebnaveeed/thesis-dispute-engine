package domain

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
)

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

type scenario struct {
	name   string
	regime Regime
	events []Event
}

// Every accepted transition of every regime is walked by at least one of these; TestTransitionCoverage enforces it.
func scenarios() []scenario {
	return []scenario{
		{
			name:   "SEPA DD: no-questions-asked refund straight from initiation",
			regime: RegimeEUSEPADirectDebit,
			events: []Event{EventIssueRefund, EventClose},
		},
		{
			name:   "SEPA DD: investigated and refunded",
			regime: RegimeEUSEPADirectDebit,
			events: []Event{EventOpenInvestigation, EventIssueRefund, EventClose},
		},
		{
			name:   "SEPA DD: questionnaire returned, then refunded",
			regime: RegimeEUSEPADirectDebit,
			events: []Event{EventOpenInvestigation, EventSendQuestionnaire, EventReceiveQuestionnaire, EventIssueRefund, EventClose},
		},
		{
			name:   "SEPA DD: denied during investigation",
			regime: RegimeEUSEPADirectDebit,
			events: []Event{EventOpenInvestigation, EventClose},
		},
		{
			name:   "SEPA DD: questionnaire never returned",
			regime: RegimeEUSEPADirectDebit,
			events: []Event{EventOpenInvestigation, EventSendQuestionnaire, EventClose},
		},
		{
			name:   "SEPA DD: denied after the questionnaire",
			regime: RegimeEUSEPADirectDebit,
			events: []Event{EventOpenInvestigation, EventSendQuestionnaire, EventReceiveQuestionnaire, EventClose},
		},
		{
			name:   "PSD2 card: investigate, fast refund, recover via chargeback, won",
			regime: RegimeEUPSD2Card,
			events: []Event{EventOpenInvestigation, EventSendQuestionnaire, EventReceiveQuestionnaire, EventIssueRefund, EventFileChargeback, EventAcknowledgeChargeback, EventSubmitEvidence, EventWinChargeback, EventClose},
		},
		{
			name:   "PSD2 card: fast refund, not recovered",
			regime: RegimeEUPSD2Card,
			events: []Event{EventOpenInvestigation, EventIssueRefund, EventClose},
		},
		{
			name:   "PSD2 card: chargeback won without evidence",
			regime: RegimeEUPSD2Card,
			events: []Event{EventOpenInvestigation, EventIssueRefund, EventFileChargeback, EventAcknowledgeChargeback, EventWinChargeback, EventClose},
		},
		{
			name:   "PSD2 card: chargeback lost on evidence",
			regime: RegimeEUPSD2Card,
			events: []Event{EventOpenInvestigation, EventIssueRefund, EventFileChargeback, EventAcknowledgeChargeback, EventSubmitEvidence, EventLoseChargeback, EventClose},
		},
		{
			name:   "PSD2 card: chargeback lost without evidence",
			regime: RegimeEUPSD2Card,
			events: []Event{EventOpenInvestigation, EventIssueRefund, EventFileChargeback, EventAcknowledgeChargeback, EventLoseChargeback, EventClose},
		},
		{
			name:   "PSD2 card: denied during investigation",
			regime: RegimeEUPSD2Card,
			events: []Event{EventOpenInvestigation, EventClose},
		},
		{
			name:   "PSD2 card: questionnaire never returned",
			regime: RegimeEUPSD2Card,
			events: []Event{EventOpenInvestigation, EventSendQuestionnaire, EventClose},
		},
		{
			name:   "PSD2 card: denied after the questionnaire",
			regime: RegimeEUPSD2Card,
			events: []Event{EventOpenInvestigation, EventSendQuestionnaire, EventReceiveQuestionnaire, EventClose},
		},
		{
			name:   "Reg E: provisional credit, chargeback won, final credit",
			regime: RegimeUSRegE,
			events: []Event{EventOpenInvestigation, EventIssueRefund, EventFileChargeback, EventAcknowledgeChargeback, EventWinChargeback, EventIssueFinalCredit, EventClose},
		},
		{
			name:   "Reg E: provisional credit, chargeback lost, credit reversed",
			regime: RegimeUSRegE,
			events: []Event{EventOpenInvestigation, EventIssueRefund, EventFileChargeback, EventAcknowledgeChargeback, EventSubmitEvidence, EventLoseChargeback, EventReverseProvisionalCredit, EventClose},
		},
		{
			name:   "Reg E: questionnaire returned, chargeback won on evidence, final credit",
			regime: RegimeUSRegE,
			events: []Event{EventOpenInvestigation, EventSendQuestionnaire, EventReceiveQuestionnaire, EventIssueRefund, EventFileChargeback, EventAcknowledgeChargeback, EventSubmitEvidence, EventWinChargeback, EventIssueFinalCredit, EventClose},
		},
		{
			name:   "Reg E: chargeback lost without evidence, credit reversed",
			regime: RegimeUSRegE,
			events: []Event{EventOpenInvestigation, EventIssueRefund, EventFileChargeback, EventAcknowledgeChargeback, EventLoseChargeback, EventReverseProvisionalCredit, EventClose},
		},
		{
			name:   "Reg E: provisional credit kept without a chargeback",
			regime: RegimeUSRegE,
			events: []Event{EventOpenInvestigation, EventIssueRefund, EventClose},
		},
		{
			name:   "Reg E: denied during investigation",
			regime: RegimeUSRegE,
			events: []Event{EventOpenInvestigation, EventClose},
		},
		{
			name:   "Reg E: questionnaire never returned",
			regime: RegimeUSRegE,
			events: []Event{EventOpenInvestigation, EventSendQuestionnaire, EventClose},
		},
		{
			name:   "Reg E: denied after the questionnaire",
			regime: RegimeUSRegE,
			events: []Event{EventOpenInvestigation, EventSendQuestionnaire, EventReceiveQuestionnaire, EventClose},
		},
		{
			name:   "Reg Z: no credit path, chargeback filed from investigation",
			regime: RegimeUSRegZ,
			events: []Event{EventOpenInvestigation, EventFileChargeback, EventAcknowledgeChargeback, EventLoseChargeback, EventClose},
		},
		{
			name:   "Reg Z: questionnaire returned, chargeback won on evidence",
			regime: RegimeUSRegZ,
			events: []Event{EventOpenInvestigation, EventSendQuestionnaire, EventReceiveQuestionnaire, EventFileChargeback, EventAcknowledgeChargeback, EventSubmitEvidence, EventWinChargeback, EventClose},
		},
		{
			name:   "Reg Z: chargeback lost on evidence",
			regime: RegimeUSRegZ,
			events: []Event{EventOpenInvestigation, EventFileChargeback, EventAcknowledgeChargeback, EventSubmitEvidence, EventLoseChargeback, EventClose},
		},
		{
			name:   "Reg Z: chargeback won without evidence",
			regime: RegimeUSRegZ,
			events: []Event{EventOpenInvestigation, EventFileChargeback, EventAcknowledgeChargeback, EventWinChargeback, EventClose},
		},
		{
			name:   "Reg Z: denied during investigation",
			regime: RegimeUSRegZ,
			events: []Event{EventOpenInvestigation, EventClose},
		},
		{
			name:   "Reg Z: questionnaire never returned",
			regime: RegimeUSRegZ,
			events: []Event{EventOpenInvestigation, EventSendQuestionnaire, EventClose},
		},
		{
			name:   "Reg Z: denied after the questionnaire",
			regime: RegimeUSRegZ,
			events: []Event{EventOpenInvestigation, EventSendQuestionnaire, EventReceiveQuestionnaire, EventClose},
		},
	}
}

func TestScenariosEndClosed(t *testing.T) {
	for _, sc := range scenarios() {
		t.Run(sc.name, func(t *testing.T) {
			if got := run(t, sc.regime, sc.events...); got.State != StateClosed {
				t.Errorf("final state = %s, want %s", got.State, StateClosed)
			}
		})
	}
}

type transition struct {
	from  State
	event Event
}

// Logs one line per regime for docs/thesis/data (scripts/eval reads it) and fails on any accepted transition no
// scenario walks. Appeals are left out; TestAppeals covers them.
func TestTransitionCoverage(t *testing.T) {
	for _, regime := range AllRegimes() {
		states := reachable(t, regime)
		accepted := map[transition]bool{}
		refused := 0
		for _, s := range states {
			for _, e := range AllEvents() {
				if e == EventAppeal {
					continue
				}
				if _, err := (Dispute{Regime: regime, State: s}).Apply(e); err == nil {
					accepted[transition{s, e}] = true
				} else {
					refused++
				}
			}
		}
		walked := map[transition]bool{}
		count := 0
		for _, sc := range scenarios() {
			if sc.regime != regime {
				continue
			}
			count++
			d := run(t, regime)
			for _, e := range sc.events {
				walked[transition{d.State, e}] = true
				next, err := d.Apply(e)
				if err != nil {
					t.Fatalf("%s: %v", sc.name, err)
				}
				d = next
			}
		}
		for tr := range accepted {
			if !walked[tr] {
				t.Errorf("%s: %s from %s is accepted but no scenario walks it", regime, tr.event, tr.from)
			}
		}
		row, err := json.Marshal(map[string]any{
			"regime": regime, "states": len(states), "accepted": len(accepted), "walked": len(walked),
			"refused": refused, "scenarios": count,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("transition-coverage %s", row)
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

// A change to these sets is a lifecycle change and belongs in docs/adr/0002.
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
		if r.HasRefundStep() != r.hasClock(DeadlineRefund) {
			t.Errorf("%s: refund step %v but refund clock %v", regime, r.HasRefundStep(), r.hasClock(DeadlineRefund))
		}
		if !r.hasClock(DeadlineResolution) {
			t.Errorf("%s: no resolution clock", regime)
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
