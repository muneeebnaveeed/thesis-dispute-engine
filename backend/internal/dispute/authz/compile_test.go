package authz_test

import (
	"strings"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/authz"
)

func TestCompile(t *testing.T) {
	src, skipped := authz.Compile(tenantID, access())
	if len(skipped) > 0 {
		t.Fatalf("skipped %v", skipped)
	}
	for _, want := range []string{
		`@id("grant:cb/junior/ISSUE_FINAL_CREDIT")`,
		`permit (principal in TeamRole::"cb/junior", action == Action::"ISSUE_FINAL_CREDIT", resource in Team::"cb")`,
		`when { resource.amount.lessThanOrEqual(decimal("500.0000")) };`,
		`@id("grant:cb/senior/ISSUE_FINAL_CREDIT")`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("compiled source lacks %q\n%s", want, src)
		}
	}
}

func TestCompileSkipsBadRows(t *testing.T) {
	a := access()
	a.Grants = append(a.Grants,
		application.Grant{Team: "cb", Role: "owner", Action: application.ActionCreate},
		application.Grant{Team: `cb") permit(principal,action,resource`, Role: application.RoleJunior, Action: application.ActionCreate})
	src, skipped := authz.Compile(tenantID, a)
	if len(skipped) != 2 || strings.Contains(src, "owner") || strings.Contains(src, "permit(principal,action,resource") {
		t.Fatalf("skipped %v\n%s", skipped, src)
	}
}
