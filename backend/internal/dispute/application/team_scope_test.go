package application_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

// twoTeams is a tenant with general (default) and fraud; Lead leads general, junior-general is a junior of it,
// fraud-only leads fraud. Grants follow the default set on general, plus create on fraud.
func twoTeams() *apptest.MemAccess {
	general := application.DefaultAccess(apptest.Lead).Grants
	return &apptest.MemAccess{
		Version: 1, Teams: []string{"fraud", "general"}, Default: "general",
		Grants: append(general, application.Grant{Team: "fraud", Role: application.RoleJunior, Action: application.ActionCreate}),
		Members: map[string][]application.Membership{
			apptest.Lead.ID:  {{Team: "general", Role: application.RoleLead}},
			"junior-general": {{Team: "general", Role: application.RoleJunior}},
			"fraud-only":     {{Team: "fraud", Role: application.RoleLead}},
		},
	}
}

func TestTeamScopeInMemory(t *testing.T) {
	svc, store := newService(t)
	store.Access = map[uuid.UUID]*apptest.MemAccess{apptest.TenantA: twoTeams()}
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "40.00")
	created, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatal(err)
	}
	outsider := apptest.CtxAs(principal.Principal{Kind: principal.Analyst, ID: "fraud-only", Display: "f@x"})
	if _, err := svc.GetDispute(outsider, created.View.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("other team read: %v", err)
	}
	if page, err := svc.ListDisputes(outsider, application.ListQuery{Limit: 50}); err != nil || len(page.Items) != 0 {
		t.Fatalf("other team list: %+v %v", page.Items, err)
	}
	key := apptest.CtxAs(principal.Principal{Kind: principal.Key, ID: uuid.NewString(), Display: "key:tk_x"})
	if _, err := svc.GetDispute(key, created.View.ID); err != nil {
		t.Fatalf("key read: %v", err)
	}
}
