package application

import (
	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
)

// Route picks a new dispute's team: the first rule whose set fields all match, else the tenant's default team.
func Route(a Access, rail domain.Rail, reason domain.Reason, tier string, amount decimal.Decimal) string {
	for _, r := range a.Routing {
		if r.Rail != nil && *r.Rail != rail {
			continue
		}
		if r.Reason != nil && *r.Reason != reason {
			continue
		}
		if r.RiskTier != nil && *r.RiskTier != tier {
			continue
		}
		if r.MinAmount != nil && amount.LessThan(*r.MinAmount) {
			continue
		}
		return r.Team
	}
	return a.DefaultTeam
}
