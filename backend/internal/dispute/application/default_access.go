package application

import (
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

// DefaultTeam is every tenant's team until its own teams are seeded.
const DefaultTeam = "general"

// DefaultAccess grants every dispute action to every analyst as a lead of the one default team, so enabling
// authorization changes nothing but the guardrails.
func DefaultAccess(p principal.Principal) Access {
	events := domain.AllEvents()
	grants := make([]Grant, 0, 5+len(events))
	grants = append(grants,
		Grant{Team: DefaultTeam, Role: RoleJunior, Action: ActionCreate},
		Grant{Team: DefaultTeam, Role: RoleJunior, Action: ActionComposeEmail},
		Grant{Team: DefaultTeam, Role: RoleJunior, Action: ActionUploadAttachment},
		Grant{Team: DefaultTeam, Role: RoleJunior, Action: ActionResendNotice},
		Grant{Team: DefaultTeam, Role: RoleLead, Action: ActionReassign},
	)
	for _, e := range events {
		grants = append(grants, Grant{Team: DefaultTeam, Role: RoleJunior, Action: EventAction(e)})
	}
	var members []Membership
	if p.Kind == principal.Analyst {
		members = []Membership{{Team: DefaultTeam, Role: RoleLead}}
	}
	return Access{DefaultTeam: DefaultTeam, Teams: []string{DefaultTeam}, Grants: grants, Members: members}
}
