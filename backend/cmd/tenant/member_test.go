//go:build integration

package main

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
)

var defaultTenant = uuid.MustParse("00000000-0000-8000-8000-00000000a001")

func TestMembers(t *testing.T) {
	pool, _ := pgtest.PoolWithSchema(t)
	ctx := context.Background()
	if err := addMember(ctx, pool, defaultTenant, "sub-1", "general", "senior"); err != nil {
		t.Fatal(err)
	}
	if err := addMember(ctx, pool, defaultTenant, "sub-1", "general", "lead"); err != nil {
		t.Fatalf("changing the role: %v", err)
	}
	if err := addMember(ctx, pool, defaultTenant, "sub-1", "nope", "lead"); err == nil {
		t.Fatal("added to an undeclared team")
	}
	if err := addMember(ctx, pool, defaultTenant, "sub-1", "general", "owner"); err == nil {
		t.Fatal("added with a role off the ladder")
	}
	got, err := listMembers(ctx, pool, defaultTenant)
	if err != nil || len(got) != 1 || got[0] != [3]string{"general", "sub-1", "lead"} {
		t.Fatalf("members = %v %v", got, err)
	}
	if n, err := removeMember(ctx, pool, defaultTenant, "sub-1", "general"); err != nil || n != 1 {
		t.Fatalf("remove: %d %v", n, err)
	}
}
