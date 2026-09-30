package authz_test

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/authz"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/golden"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

// disputeActions are the actions whose resource is a dispute, in a fixed order for the tables.
func disputeActions() []application.Action {
	var out []application.Action
	for _, e := range domain.AllEvents() {
		out = append(out, application.EventAction(e))
	}
	return append(out, application.ActionComposeEmail, application.ActionUploadAttachment, application.ActionResendNotice, application.ActionReassign)
}

// The seed fixtures' compiled policies and full decision tables are pinned, so any change to a grant or a guardrail
// is a reviewable diff of who may now do what.
func TestSeedFixtureGoldens(t *testing.T) {
	e, err := authz.New(slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob("../../../cmd/seed/access/*.yaml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("fixtures: %v %v", paths, err)
	}
	for _, path := range paths {
		slug := strings.TrimSuffix(filepath.Base(path), ".yaml")
		b, err := os.ReadFile(path) //nolint:gosec // the repo's own seed fixtures
		if err != nil {
			t.Fatal(err)
		}
		f, err := authz.ParseFixture(b)
		if err != nil {
			t.Fatal(err)
		}
		a := f.Access()
		src, skipped := authz.Compile(uuid.Nil, a)
		if len(skipped) > 0 {
			t.Fatalf("%s skipped %v", slug, skipped)
		}
		golden.Check(t, slug+".cedar.golden", []byte(src))
		golden.Check(t, slug+".decisions.golden", decisionTable(t, e, a))
	}
}

func decisionTable(t *testing.T, e *authz.Engine, a application.Access) []byte {
	t.Helper()
	var b strings.Builder
	roles := append([]application.Role{"none"}, application.Roles...)
	for _, team := range a.Teams {
		other := ""
		for _, o := range a.Teams {
			if o != team {
				other = o
				break
			}
		}
		for _, rel := range []string{"own", "other"} {
			for _, role := range roles {
				if rel == "other" && role != application.RoleLead {
					continue
				}
				access := a
				access.Members = nil
				if role != "none" {
					in := team
					if rel == "other" {
						in = other
					}
					access.Members = []application.Membership{{Team: in, Role: role}}
				}
				p := principal.Principal{Kind: principal.Analyst, ID: "caller"}
				row := func(action application.Action, amount string, d *application.DisputeFacts) {
					dec, err := e.Decide(uuid.Nil, p, access, action, d)
					if err != nil {
						t.Fatal(err)
					}
					verdict := "deny"
					if dec.Allowed {
						verdict = "allow"
					}
					fmt.Fprintf(&b, "team=%s rel=%s role=%s action=%s amount=%s -> %s %s\n", team, rel, role, action, amount, verdict, strings.Join(dec.Policies, ","))
				}
				row(application.ActionCreate, "-", &application.DisputeFacts{Team: team})
				for _, action := range disputeActions() {
					for _, amount := range []string{"100.00", "900.00"} {
						row(action, amount, &application.DisputeFacts{ID: uuid.MustParse("00000000-0000-8000-8000-000000000001"), Team: team,
							Amount: decimal.RequireFromString(amount), Currency: "EUR", Rail: domain.RailCard, State: domain.StateInvestigating,
							InvestigationOpenedBy: "someone-else"})
					}
				}
			}
		}
	}
	return []byte(b.String())
}

func benchmarkAccess(b *testing.B) application.Access {
	b.Helper()
	raw, err := os.ReadFile("../../../cmd/seed/access/otp.yaml")
	if err != nil {
		b.Fatal(err)
	}
	f, err := authz.ParseFixture(raw)
	if err != nil {
		b.Fatal(err)
	}
	a := f.Access()
	a.Members = []application.Membership{{Team: "chargebacks", Role: application.RoleSenior}}
	return a
}

// BenchmarkDecide is one decision against a cached policy set, the cost every request pays.
func BenchmarkDecide(b *testing.B) {
	e, _ := authz.New(slog.Default())
	a := benchmarkAccess(b)
	p := principal.Principal{Kind: principal.Analyst, ID: "caller"}
	d := &application.DisputeFacts{ID: uuid.New(), Team: "chargebacks", Amount: decimal.RequireFromString("900.00"), Currency: "EUR",
		Rail: domain.RailCard, State: domain.StateChargebackWon, InvestigationOpenedBy: "someone-else"}
	for b.Loop() {
		if _, err := e.Decide(uuid.Nil, p, a, application.EventAction(domain.EventIssueFinalCredit), d); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDecideCold recompiles the tenant's policy set every time, the cost of the first request after a grant changes.
func BenchmarkDecideCold(b *testing.B) {
	e, _ := authz.New(slog.Default())
	a := benchmarkAccess(b)
	p := principal.Principal{Kind: principal.Analyst, ID: "caller"}
	d := &application.DisputeFacts{ID: uuid.New(), Team: "chargebacks", Amount: decimal.RequireFromString("900.00"), Currency: "EUR",
		Rail: domain.RailCard, State: domain.StateChargebackWon, InvestigationOpenedBy: "someone-else"}
	for b.Loop() {
		a.Version++
		if _, err := e.Decide(uuid.Nil, p, a, application.EventAction(domain.EventIssueFinalCredit), d); err != nil {
			b.Fatal(err)
		}
	}
}
