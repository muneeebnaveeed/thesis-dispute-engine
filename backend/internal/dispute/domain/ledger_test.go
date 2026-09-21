package domain

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestCreditAmountAppliesTheRegimeCap(t *testing.T) {
	psd2, _ := RulesFor(RegimeEUPSD2Card)
	sepa, _ := RulesFor(RegimeEUSEPADirectDebit)
	cases := []struct {
		name      string
		r         Rules
		disputed  string
		liability string
		want      string
		fails     bool
	}{
		{"no liability", psd2, "125.40", "0", "125.40", false},
		{"within the cap", psd2, "125.40", "50", "75.40", false},
		{"over the cap", psd2, "125.40", "50.01", "", true},
		{"negative", psd2, "125.40", "-1", "", true},
		{"whole claim", psd2, "30", "30", "", true},
		{"regime without a cap", sepa, "29.90", "1", "", true},
	}
	for _, c := range cases {
		got, err := CreditAmount(c.r, dec(c.disputed), dec(c.liability))
		if c.fails {
			if !errors.Is(err, ErrInvalidLiability) {
				t.Errorf("%s: err = %v, want ErrInvalidLiability", c.name, err)
			}
			continue
		}
		if err != nil || !got.Equal(dec(c.want)) {
			t.Errorf("%s: %s %v, want %s", c.name, got, err, c.want)
		}
	}
}

func TestParseSettlement(t *testing.T) {
	sepa, _ := RulesFor(RegimeEUSEPADirectDebit)
	psd2, _ := RulesFor(RegimeEUPSD2Card)
	if s, _ := ParseSettlement(sepa, ""); s != SettleRecovered {
		t.Errorf("SEPA default = %s", s)
	}
	if s, _ := ParseSettlement(psd2, ""); s != SettleWrittenOff {
		t.Errorf("PSD2 default = %s", s)
	}
	if s, _ := ParseSettlement(psd2, "RECOVERED"); s != SettleRecovered {
		t.Errorf("explicit = %s", s)
	}
	if _, err := ParseSettlement(psd2, "MAYBE"); !errors.Is(err, ErrInvalidSettlement) {
		t.Errorf("bad word: %v", err)
	}
}

// walk applies events from INITIATED, posting as the service would, and returns the ledger.
func walk(t *testing.T, regime Regime, liability string, settlement SuspenseSettlement, events ...Event) []Posting {
	t.Helper()
	r, _ := RulesFor(regime)
	d, _ := New(regime)
	settlement, err := ParseSettlement(r, string(settlement))
	if err != nil {
		t.Fatal(err)
	}
	ledger := make([]Posting, 0, len(events))
	for _, e := range events {
		next, err := d.Apply(e)
		if err != nil {
			t.Fatalf("%s: %s from %s: %v", regime, e, d.State, err)
		}
		ledger = append(ledger, Postings(r, PostingInput{
			Entered: next.State, Disputed: dec("100"), Currency: r.Currency, Liability: dec(liability),
			Outstanding: SuspenseBalance(ledger), Settlement: settlement,
		})...)
		d = next
	}
	return ledger
}

func kinds(ps []Posting) []PostingKind {
	out := make([]PostingKind, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Kind)
	}
	return out
}

func TestPostingsOnTheMainPaths(t *testing.T) {
	// Reg E, chargeback lost: the advance comes back from the customer, nothing is recovered or absorbed.
	lost := walk(t, RegimeUSRegE, "0", "", EventOpenInvestigation, EventIssueRefund, EventFileChargeback,
		EventAcknowledgeChargeback, EventLoseChargeback, EventReverseProvisionalCredit, EventClose)
	if got := kinds(lost); len(got) != 2 || got[0] != PostingProvisionalCredit || got[1] != PostingProvisionalCreditReversal {
		t.Errorf("Reg E lost: %v", got)
	}
	if lost[1].Debit != AccountCustomer || lost[1].Credit != AccountSuspense || !lost[1].Amount.Equal(dec("100")) {
		t.Errorf("reversal = %+v", lost[1])
	}

	// Reg E, chargeback won: recovery clears suspense; the final credit itself moves nothing.
	won := walk(t, RegimeUSRegE, "50", "", EventOpenInvestigation, EventIssueRefund, EventFileChargeback,
		EventAcknowledgeChargeback, EventWinChargeback, EventIssueFinalCredit, EventClose)
	if got := kinds(won); len(got) != 2 || got[1] != PostingRecovery || !won[0].Amount.Equal(dec("50")) || !won[1].Amount.Equal(dec("50")) {
		t.Errorf("Reg E won with liability: %v %+v", got, won)
	}

	// PSD2, chargeback lost: the bank refunded and absorbs it.
	psd2Lost := walk(t, RegimeEUPSD2Card, "0", "", EventOpenInvestigation, EventIssueRefund, EventFileChargeback,
		EventAcknowledgeChargeback, EventLoseChargeback, EventClose)
	if got := kinds(psd2Lost); len(got) != 2 || got[0] != PostingFastRefund || got[1] != PostingWriteOff {
		t.Errorf("PSD2 lost: %v", got)
	}

	// SEPA: refund, then close; the scheme recovers it from the creditor by default.
	sepa := walk(t, RegimeEUSEPADirectDebit, "0", "", EventOpenInvestigation, EventIssueRefund, EventClose)
	if got := kinds(sepa); len(got) != 2 || got[0] != PostingNQARefund || got[1] != PostingRecovery {
		t.Errorf("SEPA: %v", got)
	}
	// ... unless the analyst says otherwise.
	sepaOff := walk(t, RegimeEUSEPADirectDebit, "0", SettleWrittenOff, EventOpenInvestigation, EventIssueRefund, EventClose)
	if got := kinds(sepaOff); got[1] != PostingWriteOff {
		t.Errorf("SEPA written off: %v", got)
	}

	// Reg Z never credits, so it never posts.
	if got := walk(t, RegimeUSRegZ, "0", "", EventOpenInvestigation, EventClose); len(got) != 0 {
		t.Errorf("Reg Z: %v", kinds(got))
	}
	// Closing without ever crediting posts nothing either.
	if got := walk(t, RegimeEUPSD2Card, "0", "", EventOpenInvestigation, EventClose); len(got) != 0 {
		t.Errorf("PSD2 denied: %v", kinds(got))
	}
}

// Every path from INITIATED to CLOSED, under every regime, leaves suspense at zero: what the bank advanced was
// either taken back, recovered or written off. Paths are enumerated from the transition table, so a new state
// or regime is covered without a new test.
func TestSuspenseBalancesOnEveryPath(t *testing.T) {
	for _, regime := range AllRegimes() {
		r, _ := RulesFor(regime)
		start, _ := New(regime)
		var paths [][]Event
		var explore func(d Dispute, path []Event, seen map[State]bool)
		explore = func(d Dispute, path []Event, seen map[State]bool) {
			if d.State == StateClosed {
				paths = append(paths, append([]Event(nil), path...))
				return
			}
			for _, e := range AllEvents() {
				n, err := d.Apply(e)
				if err != nil || e == EventAppeal || seen[n.State] {
					continue
				}
				seen[n.State] = true
				explore(n, append(path, e), seen)
				delete(seen, n.State)
			}
		}
		explore(start, nil, map[State]bool{start.State: true})
		if len(paths) == 0 {
			t.Fatalf("%s: no path to CLOSED", regime)
		}
		for _, path := range paths {
			for _, settlement := range []SuspenseSettlement{"", SettleRecovered, SettleWrittenOff} {
				ledger := walk(t, regime, "0", settlement, path...)
				if bal := SuspenseBalance(ledger); !bal.IsZero() {
					t.Errorf("%s via %v (settle %q): suspense %s after close; ledger %v", regime, path, settlement, bal, kinds(ledger))
				}
				customer := decimal.Zero
				for _, p := range ledger {
					if p.Credit == AccountCustomer {
						customer = customer.Add(p.Amount)
					}
					if p.Debit == AccountCustomer {
						customer = customer.Sub(p.Amount)
					}
					if p.Currency != r.Currency {
						t.Errorf("%s: posting in %s", regime, p.Currency)
					}
				}
				if customer.IsNegative() {
					t.Errorf("%s via %v: customer ends %s down", regime, path, customer)
				}
			}
		}
		t.Logf("%s: %d paths to CLOSED balance", regime, len(paths))
	}
}
