package domain

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// Account is one side of a posting in the dispute ledger (docs/adr/0014). The ledger is the issuer's view: the
// customer's account, a suspense account that holds what the bank has advanced on the dispute, and the two
// accounts that settle suspense when the dispute ends.
type Account string

// Ledger accounts.
const (
	AccountCustomer Account = "CUSTOMER" // the disputing customer's account
	AccountSuspense Account = "SUSPENSE" // dispute receivable: credits advanced and not yet recovered or absorbed
	AccountRecovery Account = "RECOVERY" // funds recovered from the network, the creditor bank or the merchant
	AccountLoss     Account = "LOSS"     // what the bank absorbs
)

// PostingKind names why money moved.
type PostingKind string

// Posting kinds.
const (
	PostingProvisionalCredit         PostingKind = "PROVISIONAL_CREDIT"
	PostingFastRefund                PostingKind = "FAST_REFUND"
	PostingNQARefund                 PostingKind = "NQA_REFUND"
	PostingProvisionalCreditReversal PostingKind = "PROVISIONAL_CREDIT_REVERSAL"
	PostingRecovery                  PostingKind = "RECOVERY"
	PostingWriteOff                  PostingKind = "WRITE_OFF"
)

// Posting is one double-entry movement: Amount leaves Credit and lands in Debit, in the bank's own books.
type Posting struct {
	Kind     PostingKind
	Debit    Account
	Credit   Account
	Amount   decimal.Decimal
	Currency string
}

// SuspenseSettlement is how an outstanding suspense balance is cleared when nothing else will clear it.
type SuspenseSettlement string

// Suspense settlements.
const (
	SettleRecovered  SuspenseSettlement = "RECOVERED"
	SettleWrittenOff SuspenseSettlement = "WRITTEN_OFF"
)

// Errors for the amounts an analyst supplies; both are the caller's to fix.
var (
	ErrInvalidLiability  = errs.New(errs.Unprocessable, "invalid-liability", "the customer liability is not allowed under this regime")
	ErrInvalidSettlement = errs.New(errs.Unprocessable, "invalid-settlement", "the settlement is not one of RECOVERED or WRITTEN_OFF")
)

// CreditAmount is what the customer receives: the disputed amount less the liability the analyst assigns, which
// the regime caps (PSD2 art. 74(1), 12 CFR 1005.6(b)) and which can never consume the whole claim.
func CreditAmount(r Rules, disputed, liability decimal.Decimal) (decimal.Decimal, error) {
	cap := decimal.NewFromInt(r.LiabilityCapMinor).Shift(-2)
	refuse := func(format string, args ...any) error {
		return ErrInvalidLiability.WithFields(errs.FieldError{Field: "body.payload.liability", Message: fmt.Sprintf(format, args...)})
	}
	switch {
	case liability.IsNegative():
		return decimal.Zero, refuse("must not be negative")
	case liability.GreaterThan(cap):
		return decimal.Zero, refuse("exceeds the %s %s cap under %s", cap.StringFixed(2), r.Currency, r.Regime)
	case liability.GreaterThanOrEqual(disputed) && disputed.IsPositive():
		return decimal.Zero, refuse("consumes the whole disputed amount of %s %s", disputed.StringFixed(2), r.Currency)
	}
	return disputed.Sub(liability), nil
}

// ParseSettlement accepts the analyst's word for how suspense clears, or the regime's default when empty.
func ParseSettlement(r Rules, raw string) (SuspenseSettlement, error) {
	switch SuspenseSettlement(raw) {
	case "":
		return r.CloseSettlement, nil
	case SettleRecovered, SettleWrittenOff:
		return SuspenseSettlement(raw), nil
	}
	return "", ErrInvalidSettlement.WithFields(errs.FieldError{Field: "body.payload.settlement", Message: fmt.Sprintf("%q is not RECOVERED or WRITTEN_OFF", raw)})
}

// PostingInput is what the ledger needs to know about the transition being applied.
type PostingInput struct {
	Entered     State
	Disputed    decimal.Decimal
	Currency    string
	Liability   decimal.Decimal    // on a refund; zero otherwise
	Outstanding decimal.Decimal    // the dispute's suspense balance before this transition
	Settlement  SuspenseSettlement // on close; the regime default when the analyst gave none
}

// Postings are the movements entering a state causes. The invariant they maintain: a dispute's suspense balance
// is zero once it is CLOSED, whichever path it took (see TestSuspenseBalancesOnEveryPath).
func Postings(r Rules, in PostingInput) []Posting {
	post := func(kind PostingKind, debit, credit Account, amount decimal.Decimal) []Posting {
		if !amount.IsPositive() {
			return nil
		}
		return []Posting{{Kind: kind, Debit: debit, Credit: credit, Amount: amount, Currency: in.Currency}}
	}
	settle := func(s SuspenseSettlement) []Posting {
		if s == SettleRecovered {
			return post(PostingRecovery, AccountRecovery, AccountSuspense, in.Outstanding)
		}
		return post(PostingWriteOff, AccountLoss, AccountSuspense, in.Outstanding)
	}
	switch in.Entered {
	case StateProvisionalCreditIssued:
		return post(PostingProvisionalCredit, AccountSuspense, AccountCustomer, in.Disputed.Sub(in.Liability))
	case StateFastRefundIssued:
		return post(PostingFastRefund, AccountSuspense, AccountCustomer, in.Disputed.Sub(in.Liability))
	case StateSEPANQARefundIssued:
		return post(PostingNQARefund, AccountSuspense, AccountCustomer, in.Disputed.Sub(in.Liability))
	case StateProvisionalCreditReversed:
		// The customer gives the advance back; suspense clears without touching recovery or loss.
		return post(PostingProvisionalCreditReversal, AccountCustomer, AccountSuspense, in.Outstanding)
	case StateChargebackWon:
		return settle(SettleRecovered)
	case StateChargebackLost:
		// A regime with provisional credit takes it back from the customer instead (the next transition).
		if r.HasProvisionalCredit() {
			return nil
		}
		return settle(SettleWrittenOff)
	case StateClosed:
		return settle(in.Settlement)
	}
	return nil
}

// SuspenseBalance sums a dispute's postings on the suspense account: debits advance, credits clear.
func SuspenseBalance(postings []Posting) decimal.Decimal {
	total := decimal.Zero
	for _, p := range postings {
		if p.Debit == AccountSuspense {
			total = total.Add(p.Amount)
		}
		if p.Credit == AccountSuspense {
			total = total.Sub(p.Amount)
		}
	}
	return total
}
