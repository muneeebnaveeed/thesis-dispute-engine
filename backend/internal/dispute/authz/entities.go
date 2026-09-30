package authz

import (
	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

func uid(typ, id string) cedar.EntityUID {
	return cedar.NewEntityUID(cedar.EntityType(typ), cedar.String(id))
}

func teamRole(team string, r application.Role) cedar.EntityUID {
	return uid("TeamRole", team+"/"+string(r))
}

func principalUID(p principal.Principal) cedar.EntityUID {
	if p.Kind == principal.Key {
		return uid("TenantKey", p.ID)
	}
	return uid("Analyst", p.ID)
}

// entities builds the request's entity store: the ladder for every team, the caller, and the resource.
func entities(p principal.Principal, a application.Access, d *application.DisputeFacts, tenant cedar.EntityUID) (cedar.EntityMap, cedar.EntityUID, error) {
	m := cedar.EntityMap{}
	for _, t := range a.Teams {
		m[teamRole(t, application.RoleLead)] = cedar.Entity{UID: teamRole(t, application.RoleLead), Parents: cedar.NewEntityUIDSet(teamRole(t, application.RoleSenior))}
		m[teamRole(t, application.RoleSenior)] = cedar.Entity{UID: teamRole(t, application.RoleSenior), Parents: cedar.NewEntityUIDSet(teamRole(t, application.RoleJunior))}
		m[teamRole(t, application.RoleJunior)] = cedar.Entity{UID: teamRole(t, application.RoleJunior)}
		m[uid("Team", t)] = cedar.Entity{UID: uid("Team", t)}
	}
	who := principalUID(p)
	switch p.Kind {
	case principal.Analyst:
		parents := make([]cedar.EntityUID, 0, len(a.Members))
		var leadOf []types.Value
		for _, mem := range a.Members {
			parents = append(parents, teamRole(mem.Team, mem.Role))
			if mem.Role == application.RoleLead {
				leadOf = append(leadOf, uid("Team", mem.Team))
			}
		}
		m[who] = cedar.Entity{UID: who, Parents: cedar.NewEntityUIDSet(parents...), Attributes: cedar.NewRecord(cedar.RecordMap{
			"id": cedar.String(p.ID), "leadOf": cedar.NewSet(leadOf...), "tenantAdmin": cedar.Boolean(p.TenantAdmin)})}
	case principal.Key:
		m[who] = cedar.Entity{UID: who, Attributes: cedar.NewRecord(cedar.RecordMap{"profile": cedar.String("")})}
	}
	m[tenant] = cedar.Entity{UID: tenant}
	if d == nil {
		return m, tenant, nil
	}
	if d.ID == uuid.Nil {
		return m, uid("Team", d.Team), nil
	}
	// an amount past Cedar's range is still decided: capped, it stays above every limit a grant can carry
	amount, err := types.ParseDecimal(decimal.Min(d.Amount, cedarMax).StringFixed(4))
	if err != nil {
		return nil, cedar.EntityUID{}, err
	}
	res := uid("Dispute", d.ID.String())
	m[res] = cedar.Entity{UID: res, Parents: cedar.NewEntityUIDSet(uid("Team", d.Team)), Attributes: cedar.NewRecord(cedar.RecordMap{
		"team": uid("Team", d.Team), "amount": amount, "currency": cedar.String(d.Currency), "rail": cedar.String(string(d.Rail)),
		"state": cedar.String(string(d.State)), "investigationOpenedBy": cedar.String(d.InvestigationOpenedBy)})}
	return m, res, nil
}
