package authz

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
)

// slugs and actions reach Cedar source as string literals; anything else is refused rather than escaped
var safe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Compile turns a tenant's grants into Cedar permits, one per row, each resource-bounded to its team. Rows it cannot
// express safely are skipped and returned, never escaped.
func Compile(_ uuid.UUID, a application.Access) (string, []string) {
	var b strings.Builder
	var skipped []string
	for _, g := range a.Grants {
		if !slices.Contains(application.Roles, g.Role) || !safe.MatchString(g.Team) || !safe.MatchString(string(g.Action)) {
			skipped = append(skipped, fmt.Sprintf("%q/%q/%q", g.Team, g.Role, g.Action))
			continue
		}
		fmt.Fprintf(&b, "@id(\"grant:%s/%s/%s\")\n", g.Team, g.Role, g.Action)
		fmt.Fprintf(&b, "permit (principal in TeamRole::\"%s/%s\", action == Action::\"%s\", resource in Team::\"%s\")", g.Team, g.Role, g.Action, g.Team)
		if g.AmountLimit != nil {
			fmt.Fprintf(&b, "\nwhen { resource.amount.lessThanOrEqual(decimal(\"%s\")) }", g.AmountLimit.StringFixed(4))
		}
		b.WriteString(";\n\n")
	}
	return b.String(), skipped
}
