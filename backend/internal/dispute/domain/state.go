package domain

import (
	"fmt"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// State is a position in the dispute lifecycle.
type State string

// Lifecycle states.
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

// AllStates lists the lifecycle in order, for callers that need the vocabulary rather than one value.
func AllStates() []State {
	return []State{
		StateInitiated, StateInvestigating, StateQuestionnaireSent, StateQuestionnaireReceived,
		StateProvisionalCreditIssued, StateFastRefundIssued, StateSEPANQARefundIssued, StateChargebackFiled,
		StateChargebackAcknowledged, StateEvidenceSubmitted, StateChargebackWon, StateChargebackLost,
		StateFinalCreditIssued, StateProvisionalCreditReversed, StateClosed,
	}
}

// ParseState accepts a state name, refusing anything outside the lifecycle.
func ParseState(raw string) (State, error) {
	for _, s := range AllStates() {
		if string(s) == raw {
			return s, nil
		}
	}
	return "", fmt.Errorf("unknown state %q", raw)
}

// Event is an occurrence that may move a dispute to a new state.
type Event string

// Events; the destination of each depends on the regime.
const (
	EventOpenInvestigation        Event = "OPEN_INVESTIGATION"
	EventSendQuestionnaire        Event = "SEND_QUESTIONNAIRE"
	EventReceiveQuestionnaire     Event = "RECEIVE_QUESTIONNAIRE"
	EventIssueRefund              Event = "ISSUE_REFUND"
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

// AllEvents lists every event.
func AllEvents() []Event {
	return []Event{
		EventOpenInvestigation, EventSendQuestionnaire, EventReceiveQuestionnaire,
		EventIssueRefund, EventFileChargeback, EventAcknowledgeChargeback,
		EventSubmitEvidence, EventWinChargeback, EventLoseChargeback,
		EventIssueFinalCredit, EventReverseProvisionalCredit, EventClose, EventAppeal,
	}
}

// Sentinel errors; match with errors.Is. Messages are user-facing; specifics are added by wrapping.
var (
	ErrInvalidTransition = errs.New(errs.Conflict, "invalid-transition", "this action is not allowed in the dispute's current state")
	ErrAppealsExhausted  = errs.New(errs.Conflict, "appeals-exhausted", "this dispute has used all of its appeals")
)

// Dispute is the lifecycle state of a dispute.
type Dispute struct {
	Regime  Regime
	State   State
	Appeals int
}

// New starts a dispute under a regime.
func New(regime Regime) (Dispute, error) {
	if _, err := RulesFor(regime); err != nil {
		return Dispute{}, err
	}
	return Dispute{Regime: regime, State: StateInitiated}, nil
}

// Apply returns the dispute after event; errors wrap ErrInvalidTransition or ErrAppealsExhausted.
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
			return d, errs.Wrap(ErrAppealsExhausted, "%d of %d used under %s", d.Appeals, r.MaxAppeals, d.Regime)
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

// IsTerminal reports whether only an appeal can move the dispute.
func (d Dispute) IsTerminal() bool { return d.State == StateClosed }

func invalid(d Dispute, e Event) error {
	return errs.Wrap(ErrInvalidTransition, "%s from %s under %s", e, d.State, d.Regime)
}

// next gates on Rules properties, never on the Regime value, so a new regime is only a new row in rules.
func next(r Rules, from State, e Event) (State, bool) {
	switch from {
	case StateInitiated:
		switch e {
		case EventOpenInvestigation:
			return StateInvestigating, true
		case EventIssueRefund:
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
			if r.HasAdjudication && !r.HasRefundStep() {
				return StateChargebackFiled, true
			}
		case EventClose:
			return StateClosed, true
		}

	case StateQuestionnaireSent:
		switch e {
		case EventReceiveQuestionnaire:
			return StateQuestionnaireReceived, true
		case EventClose:
			return StateClosed, true
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
			return StateClosed, true
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
		// Appeal is handled in Apply because it needs the counter.
	}
	return "", false
}
