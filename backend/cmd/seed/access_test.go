package main

import (
	"testing"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/authz"
)

func TestFixturesParseAndCompile(t *testing.T) {
	for _, slug := range []string{"otp", "erste"} {
		f, err := accessFixture(slug)
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		if _, skipped := authz.Compile(uuid.Nil, f.Access()); len(skipped) > 0 {
			t.Fatalf("%s skipped grants: %v", slug, skipped)
		}
		if len(f.Members) == 0 {
			t.Fatalf("%s has no members", slug)
		}
	}
}
