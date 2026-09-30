package application

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

// Role is a rung on a team's ladder; each rung has every grant of the rungs below it.
type Role string

// The ladder, fixed across tenants.
const (
	RoleJunior Role = "junior"
	RoleSenior Role = "senior"
	RoleLead   Role = "lead"
)

// Roles is the ladder, lowest first.
var Roles = []Role{RoleJunior, RoleSenior, RoleLead}

// Action is what a principal asks to do; dispute events are actions under their own names.
type Action string

// Operations beyond the dispute events.
const (
	ActionCreate           Action = "create"
	ActionComposeEmail     Action = "compose-email"
	ActionUploadAttachment Action = "upload-attachment"
	ActionResendNotice     Action = "resend-notice"
	ActionReassign         Action = "reassign"
	ActionManageKeys       Action = "manage-keys"
	ActionManageTemplates  Action = "manage-templates"
	ActionManageBranding   Action = "manage-branding"
)

// EventAction is the action that applying e requires.
func EventAction(e domain.Event) Action { return Action(e) }

// Grant lets a role in a team take an action on that team's disputes, up to an amount when one is set.
type Grant struct {
	Team        string
	Role        Role
	Action      Action
	AmountLimit *decimal.Decimal
}

// Membership places the caller in a team at a role.
type Membership struct {
	Team string
	Role Role
}

// RoutingRule sends a new dispute to a team when every set field matches.
type RoutingRule struct {
	Rail      *domain.Rail
	Reason    *domain.Reason
	RiskTier  *string
	MinAmount *decimal.Decimal
	Team      string
}

// Access is a tenant's teams, grants and routing at one policy version, plus the caller's memberships.
type Access struct {
	Version     int64
	DefaultTeam string
	Teams       []string
	Grants      []Grant
	Routing     []RoutingRule
	Members     []Membership
}

// DisputeFacts is what policies may read about a dispute. A zero ID means the team a new dispute routes to.
type DisputeFacts struct {
	ID                    uuid.UUID
	Team                  string
	Amount                decimal.Decimal
	Currency              string
	Rail                  domain.Rail
	State                 domain.State
	InvestigationOpenedBy string
}

// Decision is the engine's answer and the policies that decided it.
type Decision struct {
	Allowed  bool
	Policies []string
}

// Authorizer decides whether p may take action; d nil means the tenant itself.
type Authorizer interface {
	Decide(tenantID uuid.UUID, p principal.Principal, a Access, action Action, d *DisputeFacts) (Decision, error)
}

// ErrForbidden is a caller whose grants or the guardrails refuse the operation.
var ErrForbidden = errs.New(errs.Forbidden, "forbidden", "your role does not allow this operation")
