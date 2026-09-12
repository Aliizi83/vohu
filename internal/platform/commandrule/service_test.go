package commandrule_test

import (
	"context"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/commandrule"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&commandrule.Rule{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) commandrule.Service {
	t.Helper()
	return commandrule.NewService(commandrule.NewRepository(setupTestDB(t)))
}

func TestCreate_DefaultsAllowedToTrueWhenOmitted(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	rule, err := service.Create(ctx, commandrule.CreateRuleRequest{
		SSHConnectionID: 1,
		Program:         "pwd",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !rule.Allowed {
		t.Fatalf("expected Allowed to default to true, got false")
	}
}

func TestCreate_RespectsExplicitAllowedFalse(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	deny := false
	rule, err := service.Create(ctx, commandrule.CreateRuleRequest{
		SSHConnectionID: 1,
		Program:         "git",
		ArgsPrefixes:    [][]string{{"push"}},
		Allowed:         &deny,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if rule.Allowed {
		t.Fatalf("expected Allowed to stay false, got true")
	}
	if len(rule.ArgsPrefixes) != 1 || len(rule.ArgsPrefixes[0]) != 1 || rule.ArgsPrefixes[0][0] != "push" {
		t.Fatalf("expected ArgsPrefixes to round-trip, got %v", rule.ArgsPrefixes)
	}
}

func TestUpdate_OmittedFieldsAreLeftUnchanged(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	created, err := service.Create(ctx, commandrule.CreateRuleRequest{
		SSHConnectionID: 1,
		Program:         "git",
		ArgsPrefixes:    [][]string{{"status"}},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updated, err := service.Update(ctx, created.ID, commandrule.UpdateRuleRequest{})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Program != "git" {
		t.Fatalf("expected Program to stay \"git\", got %q", updated.Program)
	}
	if len(updated.ArgsPrefixes) != 1 || updated.ArgsPrefixes[0][0] != "status" {
		t.Fatalf("expected ArgsPrefixes to stay [[\"status\"]], got %v", updated.ArgsPrefixes)
	}
	if !updated.Allowed {
		t.Fatalf("expected Allowed to stay true, got false")
	}
}

// TestUpdate_ExplicitEmptyArgsPrefixesClearsIt is the one case a plain
// zero-value check can't express — an explicit "argsPrefixes: []" has to
// be distinguishable from an omitted field, since one means "match any
// args" and the other means "don't touch this."
func TestUpdate_ExplicitEmptyArgsPrefixesClearsIt(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	created, err := service.Create(ctx, commandrule.CreateRuleRequest{
		SSHConnectionID: 1,
		Program:         "git",
		ArgsPrefixes:    [][]string{{"status"}},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updated, err := service.Update(ctx, created.ID, commandrule.UpdateRuleRequest{
		ArgsPrefixes: [][]string{},
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if len(updated.ArgsPrefixes) != 0 {
		t.Fatalf("expected ArgsPrefixes to be cleared, got %v", updated.ArgsPrefixes)
	}
}

func TestUpdate_AllowedFalseIsRespected(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	created, err := service.Create(ctx, commandrule.CreateRuleRequest{SSHConnectionID: 1, Program: "pwd"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	deny := false
	updated, err := service.Update(ctx, created.ID, commandrule.UpdateRuleRequest{Allowed: &deny})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Allowed {
		t.Fatalf("expected Allowed to become false, got true")
	}
}

func TestListForConnection_ScopedToOneConnection(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	if _, err := service.Create(ctx, commandrule.CreateRuleRequest{SSHConnectionID: 1, Program: "pwd"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := service.Create(ctx, commandrule.CreateRuleRequest{SSHConnectionID: 1, Program: "ls"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := service.Create(ctx, commandrule.CreateRuleRequest{SSHConnectionID: 2, Program: "whoami"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rules, total, err := service.ListForConnection(ctx, 1, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListForConnection failed: %v", err)
	}
	if total != 2 || len(rules) != 2 {
		t.Fatalf("expected 2 rules for connection 1, got total=%d len=%d", total, len(rules))
	}
	for _, r := range rules {
		if r.SSHConnectionID != 1 {
			t.Fatalf("expected every rule to belong to connection 1, got connection %d", r.SSHConnectionID)
		}
	}
}

func TestDelete_RemovesTheRule(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	created, err := service.Create(ctx, commandrule.CreateRuleRequest{SSHConnectionID: 1, Program: "pwd"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := service.GetByID(ctx, created.ID); err != shared.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
