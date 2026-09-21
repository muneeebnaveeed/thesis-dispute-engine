package domain

// NoticeKind is a communication the engine owes the customer at a point in the lifecycle (docs/adr/0017).
type NoticeKind string

// Notice kinds.
const (
	NoticeAcknowledgement   NoticeKind = "ACKNOWLEDGEMENT"    // the dispute was received
	NoticeQuestionnaire     NoticeKind = "QUESTIONNAIRE"      // the questions the customer must answer
	NoticeProvisionalCredit NoticeKind = "PROVISIONAL_CREDIT" // a reversible credit was posted
	NoticeRefund            NoticeKind = "REFUND"             // a refund was posted
	NoticeReversal          NoticeKind = "REVERSAL"           // the provisional credit was taken back
	NoticeResolution        NoticeKind = "RESOLUTION"         // the outcome, and how it was reached
)

// Channel is how a notice reaches the customer.
type Channel string

// Channels.
const (
	ChannelEmail  Channel = "EMAIL"
	ChannelLetter Channel = "LETTER" // a printable document; the written notice some regimes require
)

// NoticesFor lists what entering a state obliges the bank to tell the customer. Gated on the state and one
// property of the rules (WrittenNotices), never on the regime's name.
func NoticesFor(r Rules, entered State) []NoticeKind {
	switch entered {
	case StateInitiated:
		return []NoticeKind{NoticeAcknowledgement}
	case StateQuestionnaireSent:
		return []NoticeKind{NoticeQuestionnaire}
	case StateProvisionalCreditIssued:
		return []NoticeKind{NoticeProvisionalCredit}
	case StateFastRefundIssued, StateSEPANQARefundIssued:
		return []NoticeKind{NoticeRefund}
	case StateProvisionalCreditReversed:
		return []NoticeKind{NoticeReversal}
	case StateClosed:
		return []NoticeKind{NoticeResolution}
	}
	return nil
}

// ChannelsFor says how a kind of notice goes out under a regime: always email; a letter as well where the
// regime's notices must be in writing and the notice carries a legal consequence (not the questionnaire).
func ChannelsFor(r Rules, kind NoticeKind) []Channel {
	if r.WrittenNotices && kind != NoticeQuestionnaire {
		return []Channel{ChannelEmail, ChannelLetter}
	}
	return []Channel{ChannelEmail}
}
