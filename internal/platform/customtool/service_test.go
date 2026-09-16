package customtool_test

import (
	"context"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/customtool"
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
	if err := db.AutoMigrate(&customtool.Tool{}, &customtool.ToolVersion{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func allowAccessLevel(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
	return true, nil
}

func denyAccessLevel(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
	return false, nil
}

func noopGrantAccess(ctx context.Context, userID uint, resourceType string, resourceID uint, level string, effect string) error {
	return nil
}

func newTestService(t *testing.T, hasAccessLevel shared.AccessLevelCheck) customtool.Service {
	t.Helper()
	return customtool.NewService(customtool.NewRepository(setupTestDB(t)), noopGrantAccess, hasAccessLevel)
}

func TestCreateTool_DefaultsVisibilityToPrivateWhenOmitted(t *testing.T) {
	service := newTestService(t, allowAccessLevel)
	ctx := context.Background()

	tool, err := service.CreateTool(ctx, 1, customtool.CreateToolRequest{
		Name: "send_file", Description: "sends a file to a URL", ParamsSchema: "{}",
	})
	if err != nil {
		t.Fatalf("CreateTool failed: %v", err)
	}
	if tool.Visibility != customtool.VisibilityPrivate {
		t.Fatalf("expected default visibility private, got %q", tool.Visibility)
	}
	if tool.CreatedByUserID != 1 {
		t.Fatalf("expected CreatedByUserID 1, got %d", tool.CreatedByUserID)
	}
}

func TestCreateTool_RespectsExplicitVisibility(t *testing.T) {
	service := newTestService(t, allowAccessLevel)
	ctx := context.Background()

	tool, err := service.CreateTool(ctx, 1, customtool.CreateToolRequest{
		Name: "send_file", Description: "sends a file to a URL", ParamsSchema: "{}",
		Visibility: customtool.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("CreateTool failed: %v", err)
	}
	if tool.Visibility != customtool.VisibilityPublic {
		t.Fatalf("expected visibility public, got %q", tool.Visibility)
	}
}

func TestUpdateTool_OmittedFieldsAreLeftUnchanged(t *testing.T) {
	service := newTestService(t, allowAccessLevel)
	ctx := context.Background()

	created, err := service.CreateTool(ctx, 1, customtool.CreateToolRequest{
		Name: "send_file", Description: "original description", ParamsSchema: `{"a":1}`,
	})
	if err != nil {
		t.Fatalf("CreateTool failed: %v", err)
	}

	updated, err := service.UpdateTool(ctx, created.ID, customtool.UpdateToolRequest{})
	if err != nil {
		t.Fatalf("UpdateTool failed: %v", err)
	}
	if updated.Description != "original description" {
		t.Fatalf("expected description to stay unchanged, got %q", updated.Description)
	}
	if updated.ParamsSchema != `{"a":1}` {
		t.Fatalf("expected paramsSchema to stay unchanged, got %q", updated.ParamsSchema)
	}
}

func TestUpdateTool_NeverChangesName(t *testing.T) {
	service := newTestService(t, allowAccessLevel)
	ctx := context.Background()

	created, err := service.CreateTool(ctx, 1, customtool.CreateToolRequest{
		Name: "send_file", Description: "d", ParamsSchema: "{}",
	})
	if err != nil {
		t.Fatalf("CreateTool failed: %v", err)
	}

	updated, err := service.UpdateTool(ctx, created.ID, customtool.UpdateToolRequest{Description: "new description"})
	if err != nil {
		t.Fatalf("UpdateTool failed: %v", err)
	}
	if updated.Name != "send_file" {
		t.Fatalf("expected name to stay \"send_file\", got %q", updated.Name)
	}
}

func TestCreateVersion_ThenLatestVersionForTool_ReturnsMostRecent(t *testing.T) {
	service := newTestService(t, allowAccessLevel)
	ctx := context.Background()

	tool, err := service.CreateTool(ctx, 1, customtool.CreateToolRequest{
		Name: "send_file", Description: "d", ParamsSchema: "{}",
	})
	if err != nil {
		t.Fatalf("CreateTool failed: %v", err)
	}

	if _, err := service.CreateVersion(ctx, 1, tool.ID, customtool.CreateVersionRequest{
		Version: "1.0.0", SourceCode: "package main\nfunc main() {}",
	}); err != nil {
		t.Fatalf("CreateVersion v1 failed: %v", err)
	}
	v2, err := service.CreateVersion(ctx, 1, tool.ID, customtool.CreateVersionRequest{
		Version: "2.0.0", SourceCode: "package main\nfunc main() { println(2) }",
	})
	if err != nil {
		t.Fatalf("CreateVersion v2 failed: %v", err)
	}

	latest, err := service.LatestVersionForTool(ctx, tool.ID)
	if err != nil {
		t.Fatalf("LatestVersionForTool failed: %v", err)
	}
	if latest.ID != v2.ID || latest.Version != "2.0.0" {
		t.Fatalf("expected latest version to be 2.0.0 (id %d), got %q (id %d)", v2.ID, latest.Version, latest.ID)
	}
}

func TestCreateVersion_FailsForNonexistentTool(t *testing.T) {
	service := newTestService(t, allowAccessLevel)
	ctx := context.Background()

	_, err := service.CreateVersion(ctx, 1, 999, customtool.CreateVersionRequest{
		Version: "1.0.0", SourceCode: "package main\nfunc main() {}",
	})
	if err != shared.ErrNotFound {
		t.Fatalf("expected ErrNotFound for a nonexistent tool, got %v", err)
	}
}

func TestListVersionsForTool_ScopedToOneTool(t *testing.T) {
	service := newTestService(t, allowAccessLevel)
	ctx := context.Background()

	toolA, err := service.CreateTool(ctx, 1, customtool.CreateToolRequest{Name: "tool_a", Description: "d", ParamsSchema: "{}"})
	if err != nil {
		t.Fatalf("CreateTool a failed: %v", err)
	}
	toolB, err := service.CreateTool(ctx, 1, customtool.CreateToolRequest{Name: "tool_b", Description: "d", ParamsSchema: "{}"})
	if err != nil {
		t.Fatalf("CreateTool b failed: %v", err)
	}

	if _, err := service.CreateVersion(ctx, 1, toolA.ID, customtool.CreateVersionRequest{Version: "1.0.0", SourceCode: "package main"}); err != nil {
		t.Fatalf("CreateVersion a1 failed: %v", err)
	}
	if _, err := service.CreateVersion(ctx, 1, toolA.ID, customtool.CreateVersionRequest{Version: "1.0.1", SourceCode: "package main"}); err != nil {
		t.Fatalf("CreateVersion a2 failed: %v", err)
	}
	if _, err := service.CreateVersion(ctx, 1, toolB.ID, customtool.CreateVersionRequest{Version: "1.0.0", SourceCode: "package main"}); err != nil {
		t.Fatalf("CreateVersion b1 failed: %v", err)
	}

	versions, total, err := service.ListVersionsForTool(ctx, toolA.ID, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListVersionsForTool failed: %v", err)
	}
	if total != 2 || len(versions) != 2 {
		t.Fatalf("expected 2 versions for tool A, got total=%d len=%d", total, len(versions))
	}
	for _, v := range versions {
		if v.ToolID != toolA.ID {
			t.Fatalf("expected every version to belong to tool A, got tool %d", v.ToolID)
		}
	}
}

func TestDeleteTool_RemovesToolAndItsVersions(t *testing.T) {
	service := newTestService(t, allowAccessLevel)
	ctx := context.Background()

	tool, err := service.CreateTool(ctx, 1, customtool.CreateToolRequest{Name: "send_file", Description: "d", ParamsSchema: "{}"})
	if err != nil {
		t.Fatalf("CreateTool failed: %v", err)
	}
	if _, err := service.CreateVersion(ctx, 1, tool.ID, customtool.CreateVersionRequest{Version: "1.0.0", SourceCode: "package main"}); err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}

	if err := service.DeleteTool(ctx, tool.ID); err != nil {
		t.Fatalf("DeleteTool failed: %v", err)
	}

	if _, err := service.GetToolByID(ctx, tool.ID); err != shared.ErrNotFound {
		t.Fatalf("expected ErrNotFound for the deleted tool, got %v", err)
	}
	if _, err := service.LatestVersionForTool(ctx, tool.ID); err != shared.ErrNotFound {
		t.Fatalf("expected ErrNotFound for the deleted tool's versions, got %v", err)
	}
}

func TestListToolsForCaller_PublicToolsVisibleToEveryone(t *testing.T) {
	service := newTestService(t, denyAccessLevel)
	ctx := context.Background()

	if _, err := service.CreateTool(ctx, 1, customtool.CreateToolRequest{
		Name: "public_tool", Description: "d", ParamsSchema: "{}", Visibility: customtool.VisibilityPublic,
	}); err != nil {
		t.Fatalf("CreateTool public failed: %v", err)
	}
	if _, err := service.CreateTool(ctx, 1, customtool.CreateToolRequest{
		Name: "private_tool", Description: "d", ParamsSchema: "{}", Visibility: customtool.VisibilityPrivate,
	}); err != nil {
		t.Fatalf("CreateTool private failed: %v", err)
	}

	tools, total, err := service.ListToolsForCaller(ctx, 2, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListToolsForCaller failed: %v", err)
	}
	if total != 1 || len(tools) != 1 || tools[0].Name != "public_tool" {
		t.Fatalf("expected only the public tool to be visible to a caller with no grants, got total=%d tools=%+v", total, tools)
	}
}
