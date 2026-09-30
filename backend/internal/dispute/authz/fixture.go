package authz

import (
	"bytes"
	"errors"
	"fmt"
	"slices"

	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
)

// Fixture is a tenant's access data as seeded: teams, every membership, grants and routing.
type Fixture struct {
	Teams   []FixtureTeam
	Members []FixtureMember
	Grants  []application.Grant
	Routing []application.RoutingRule
}

// FixtureTeam is one team and whether new disputes default to it.
type FixtureTeam struct {
	Slug    string
	Name    string
	Default bool
}

// FixtureMember places a subject in a team.
type FixtureMember struct {
	Subject string
	Team    string
	Role    application.Role
}

type fixtureDoc struct {
	Teams []struct {
		Slug    string `yaml:"slug"`
		Name    string `yaml:"name"`
		Default bool   `yaml:"default"`
	} `yaml:"teams"`
	Members []struct {
		Subject string `yaml:"subject"`
		Team    string `yaml:"team"`
		Role    string `yaml:"role"`
	} `yaml:"members"`
	Grants []struct {
		Team        string   `yaml:"team"`
		Role        string   `yaml:"role"`
		Actions     []string `yaml:"actions"`
		AmountLimit string   `yaml:"amount_limit"`
	} `yaml:"grants"`
	Routing []struct {
		Rail      string `yaml:"rail"`
		Reason    string `yaml:"reason"`
		RiskTier  string `yaml:"risk_tier"`
		MinAmount string `yaml:"min_amount"`
		Team      string `yaml:"team"`
	} `yaml:"routing"`
}

// ParseFixture reads a fixture strictly: unknown fields, undeclared teams, roles off the ladder and a missing default
// team are all refused, so a typo fails the seed instead of silently granting less.
func ParseFixture(b []byte) (Fixture, error) {
	var doc fixtureDoc
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return Fixture{}, fmt.Errorf("authz: fixture: %w", err)
	}
	var f Fixture
	var teams []string
	defaults := 0
	for _, t := range doc.Teams {
		f.Teams = append(f.Teams, FixtureTeam{Slug: t.Slug, Name: t.Name, Default: t.Default})
		teams = append(teams, t.Slug)
		if t.Default {
			defaults++
		}
	}
	if defaults != 1 {
		return Fixture{}, errors.New("authz: fixture: exactly one team must be the default")
	}
	known := func(team string) error {
		if !slices.Contains(teams, team) {
			return fmt.Errorf("authz: fixture: team %q is not declared", team)
		}
		return nil
	}
	role := func(r string) (application.Role, error) {
		if !slices.Contains(application.Roles, application.Role(r)) {
			return "", fmt.Errorf("authz: fixture: role %q is not on the ladder", r)
		}
		return application.Role(r), nil
	}
	amount := func(s string) (*decimal.Decimal, error) {
		if s == "" {
			return nil, nil
		}
		d, err := decimal.NewFromString(s)
		if err != nil {
			return nil, fmt.Errorf("authz: fixture: amount %q: %w", s, err)
		}
		return &d, nil
	}
	for _, m := range doc.Members {
		r, err := role(m.Role)
		if err != nil {
			return Fixture{}, err
		}
		if err := known(m.Team); err != nil {
			return Fixture{}, err
		}
		f.Members = append(f.Members, FixtureMember{Subject: m.Subject, Team: m.Team, Role: r})
	}
	for _, g := range doc.Grants {
		r, err := role(g.Role)
		if err != nil {
			return Fixture{}, err
		}
		if err := known(g.Team); err != nil {
			return Fixture{}, err
		}
		limit, err := amount(g.AmountLimit)
		if err != nil {
			return Fixture{}, err
		}
		for _, a := range g.Actions {
			f.Grants = append(f.Grants, application.Grant{Team: g.Team, Role: r, Action: application.Action(a), AmountLimit: limit})
		}
	}
	for _, rr := range doc.Routing {
		if err := known(rr.Team); err != nil {
			return Fixture{}, err
		}
		min, err := amount(rr.MinAmount)
		if err != nil {
			return Fixture{}, err
		}
		rule := application.RoutingRule{Team: rr.Team, MinAmount: min}
		if rr.Rail != "" {
			rail := domain.Rail(rr.Rail)
			rule.Rail = &rail
		}
		if rr.Reason != "" {
			reason := domain.Reason(rr.Reason)
			rule.Reason = &reason
		}
		if rr.RiskTier != "" {
			tier := rr.RiskTier
			rule.RiskTier = &tier
		}
		f.Routing = append(f.Routing, rule)
	}
	return f, nil
}

// Access is the fixture as the engine sees a tenant, without any caller's memberships.
func (f Fixture) Access() application.Access {
	a := application.Access{Grants: f.Grants, Routing: f.Routing}
	for _, t := range f.Teams {
		a.Teams = append(a.Teams, t.Slug)
		if t.Default {
			a.DefaultTeam = t.Slug
		}
	}
	return a
}
