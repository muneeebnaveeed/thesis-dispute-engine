package dispute

import (
	"errors"
	"fmt"
)

// State is a position in the dispute lifecycle. Which states a dispute can
// reach depends on its regime; see next.
type State string

// Lifecycle states, in the order they typically occur.
const (
	StateInitiated                 State = "INITIATED"
	StateInvestigating             State = "INVESTIGATING"
	StateQuestionnaireSent         State = "QUESTIONNAIRE_SENT"
	StateQuestionnaireReceived     State = "QUESTIONNAIRE_RECEIVED"
	StateProvisionalCreditIssued   State = "PROVISIONAL_CREDIT_ISSUED"
	StateFastRefundIssued          State = "FAST_REFUND_ISSUED"
	StateSEPANQARefundIssued       State = "SEPA_NQA_REFUND_ISSUED"
	StateChargebackFiled           State = "CHARGEBACK_FILED"
	StateChargebackAcknowledged    State = "CHARGEBACK_ACKNOWLEDGED"
	StateEvidenceSubmitted         State = "EVIDENCE_SUBMITTED"
	StateChargebackWon             State = "CHARGEBACK_WON"
	StateChargebackLost            State = "CHARGEBACK_LOST"
	StateFinalCreditIssued         State = "FINAL_CREDIT_ISSUED"
	StateProvisionalCreditReversed State = "PROVISIONAL_CREDIT_REVERSED"
	StateClosed                    State = "CLOSED"
)

// Event is something that happens to a dispute and may move it to a new state.
type Event string

// Events. Names are what happened, not where it goes; the destination depends
// on the regime.
const (
	EventOpenInvestigation        Event = "OPEN_INVESTIGATION"
	EventSendQuestionnaire        Event = "SEND_QUESTIONNAIRE"
	EventReceiveQuestionnaire     Event = "RECEIVE_QUESTIONNAIRE"
	EventIssueRefund              Event = "ISSUE_REFUND" // the regime's refund path
	EventFileChargeback           Event = "FILE_CHARGEBACK"
	EventAcknowledgeChargeback    Event = "ACKNOWLEDGE_CHARGEBACK"
	EventSubmitEvidence           Event = "SUBMIT_EVIDENCE"
	EventWinChargeback            Event = "WIN_CHARGEBACK"
	EventLoseChargeback           Event = "LOSE_CHARGEBACK"
	EventIssueFinalCredit         Event = "ISSUE_FINAL_CREDIT"
	EventReverseProvisionalCredit Event = "REVERSE_PROVISIONAL_CREDIT"
	EventClose                    Event = "CLOSE"
	EventAppeal                   Event = "APPEAL"
)

// AllEvents lists every event, for enumeration in tests and API docs.
func AllEvents() []Event {
	return []Event{
		EventOpenInvestigation, EventSendQuestionnaire, EventReceiveQuestionnaire,
		EventIssueRefund, EventFileChargeback, EventAcknowledgeChargeback,
		EventSubmitEvidence, EventWinChargeback, EventLoseChargeback,
		EventIssueFinalCredit, EventReverseProvisionalCredit, EventClose, EventAppeal,
	}
}

// Sentinel errors. Callers match with errors.Is; the message carries detail.
var (
	ErrInvalidTransition = errors.New("dispute: transition not allowed")
	ErrAppealsExhausted  = errors.New("dispute: appeals exhausted")
)

// Dispute is the lifecycle-relevant part of a dispute: enough to decide which
// events are allowed. Persistence, amounts and parties live elsewhere.
type Dispute struct {
	Regime  Regime
	State   State
	Appeals int // appeals already used
}

// New starts a dispute under a regime.
func New(regime Regime) (Dispute, error) {
	if _, err := RulesFor(regime); err != nil {
		return Dispute{}, err
	}
	return Dispute{Regime: regime, State: StateInitiated}, nil
}

// Apply returns the dispute after event, or an error that wraps
// ErrInvalidTransition or ErrAppealsExhausted. Dispute is a value; the receiver
// is never mutated.
func (d Dispute) Apply(event Event) (Dispute, error) {
	r, err := RulesFor(d.Regime)
	if err != nil {
		return d, err
	}
	if event == EventAppeal {
		if d.State != StateClosed {
			return d, invalid(d, event)
		}
		if d.Appeals >= r.MaxAppeals {
			return d, fmt.Errorf("%w: %d of %d used under %s", ErrAppealsExhausted, d.Appeals, r.MaxAppeals, d.Regime)
		}
		return Dispute{Regime: d.Regime, State: StateInvestigating, Appeals: d.Appeals + 1}, nil
	}
	to, ok := next(r, d.State, event)
	if !ok {
		return d, invalid(d, event)
	}
	return Dispute{Regime: d.Regime, State: to, Appeals: d.Appeals}, nil
}

// Allowed lists the events Apply would accept from the current state.
func (d Dispute) Allowed() []Event {
	var out []Event
	for _, e := range AllEvents() {
		if _, err := d.Apply(e); err == nil {
			out = append(out, e)
		}
	}
	return out
}

// IsTerminal reports whether only an appeal can move the dispute on.
func (d Dispute) IsTerminal() bool { return d.State == StateClosed }

func invalid(d Dispute, e Event) error {
	return fmt.Errorf("%w: %s from %s under %s", ErrInvalidTransition, e, d.State, d.Regime)
}

// next is the transition table. Every arm that depends on the regime gates on
// a Rules property, never on the Regime value, so a new regime is a new row in
// rules and nothing here.
func next(r Rules, from State, e Event) (State, bool) {
	switch from {
	case StateInitiated:
		switch e {
		case EventOpenInvestigation:
			return StateInvestigating, true
		case EventIssueRefund:
			// No-questions-asked regimes refund straight away; everything
			// else investigates first.
			if r.Refund == RefundNoQuestionsAsked {
				return StateSEPANQARefundIssued, true
			}
		}

	case StateInvestigating:
		switch e {
		case EventSendQuestionnaire:
			return StateQuestionnaireSent, true
		case EventIssueRefund:
			if r.HasRefundStep() {
				return r.refundState(), true
			}
		case EventFileChargeback:
			// Regimes without a refund step go to the network directly.
			if r.HasAdjudication && !r.HasRefundStep() {
				return StateChargebackFiled, true
			}
		case EventClose:
			return StateClosed, true // denied or withdrawn
		}

	case StateQuestionnaireSent:
		switch e {
		case EventReceiveQuestionnaire:
			return StateQuestionnaireReceived, true
		case EventClose:
			return StateClosed, true // abandoned by the customer
		}

	case StateQuestionnaireReceived:
		switch e {
		case EventIssueRefund:
			if r.HasRefundStep() {
				return r.refundState(), true
			}
		case EventFileChargeback:
			if r.HasAdjudication && !r.HasRefundStep() {
				return StateChargebackFiled, true
			}
		case EventClose:
			return StateClosed, true
		}

	case StateProvisionalCreditIssued, StateFastRefundIssued, StateSEPANQARefundIssued:
		switch e {
		case EventFileChargeback:
			if r.HasAdjudication {
				return StateChargebackFiled, true
			}
		case EventClose:
			return StateClosed, true // issuer absorbs, or nothing to recover
		}

	case StateChargebackFiled:
		if e == EventAcknowledgeChargeback {
			return StateChargebackAcknowledged, true
		}

	case StateChargebackAcknowledged:
		switch e {
		case EventSubmitEvidence:
			if r.MerchantMayContest {
				return StateEvidenceSubmitted, true
			}
		case EventWinChargeback:
			return StateChargebackWon, true
		case EventLoseChargeback:
			return StateChargebackLost, true
		}

	case StateEvidenceSubmitted:
		switch e {
		case EventWinChargeback:
			return StateChargebackWon, true
		case EventLoseChargeback:
			return StateChargebackLost, true
		}

	case StateChargebackWon:
		switch e {
		case EventIssueFinalCredit:
			if r.HasProvisionalCredit() {
				return StateFinalCreditIssued, true
			}
		case EventClose:
			if !r.HasProvisionalCredit() {
				return StateClosed, true
			}
		}

	case StateChargebackLost:
		switch e {
		case EventReverseProvisionalCredit:
			if r.HasProvisionalCredit() {
				return StateProvisionalCreditReversed, true
			}
		case EventClose:
			if !r.HasProvisionalCredit() {
				return StateClosed, true
			}
		}

	case StateFinalCreditIssued, StateProvisionalCreditReversed:
		if e == EventClose {
			return StateClosed, true
		}

	case StateClosed:
		// Only EventAppeal, handled in Apply because it needs the counter.
	}
	return "", false
}
