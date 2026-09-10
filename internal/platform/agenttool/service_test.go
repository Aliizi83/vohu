package agenttool_test

import (
	"context"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/agenttool"
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
	if err := db.AutoMigrate(&agenttool.Tool{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func denyAllLevels(context.Context, uint, string, uint, string) (bool, error) { return false, nil }

func allowAllLevels(context.Context, uint, string, uint, string) (bool, error) { return true, nil }

// seedTool inserts a row directly through the repository — there's no
// Service.Create (see Tool's doc comment: the catalog has no
// user-created rows, only seeders.seedAgentTools writes them), so tests
// that need a row to exist go around Service the same way the real
// seeder does.
func seedTool(t *testing.T, repo agenttool.Repository, name string, visibility agenttool.Visibility) agenttool.Tool {
	t.Helper()
	tool := &agenttool.Tool{Name: name, Description: "d", Visibility: visibility}
	if err := repo.Create(context.Background(), tool); err != nil {
		t.Fatalf("seed Create failed: %v", err)
	}
	return *tool
}

func TestListForCaller_PublicToolIsVisibleWithoutAnyGrant(t *testing.T) {
	repo := agenttool.NewRepository(setupTestDB(t))
	seedTool(t, repo, "ssh_execute", agenttool.VisibilityPublic)

	service := agenttool.NewService(repo, denyAllLevels) // caller has NO grants anywhere
	items, total, err := service.ListForCaller(context.Background(), 999, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListForCaller failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected the public tool to be visible with zero grants, got total=%d len=%d", total, len(items))
	}
}

func TestListForCaller_PrivateToolIsHiddenWithoutAGrant(t *testing.T) {
	repo := agenttool.NewRepository(setupTestDB(t))
	seedTool(t, repo, "read_file", agenttool.VisibilityPrivate)

	service := agenttool.NewService(repo, denyAllLevels)
	items, total, err := service.ListForCaller(context.Background(), 999, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListForCaller failed: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("expected a private tool with no grant to be invisible, got total=%d len=%d (%+v)", total, len(items), items)
	}
}

func TestListForCaller_PrivateToolIsVisibleToASpecificallyGrantedUser(t *testing.T) {
	repo := agenttool.NewRepository(setupTestDB(t))
	tool := seedTool(t, repo, "read_file", agenttool.VisibilityPrivate)

	granted := agenttool.NewService(repo, func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
		return resourceID == tool.ID, nil
	})

	items, total, err := granted.ListForCaller(context.Background(), 42, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListForCaller failed: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != tool.ID {
		t.Fatalf("expected the specifically-granted private tool to be visible, got total=%d items=%+v", total, items)
	}
}

func TestListForCaller_WildcardAccessSeesEveryPrivateTool(t *testing.T) {
	// The "higher-level users can reach lower-level tools" requirement —
	// a wildcard "read"/"manage" grant (what seedAdminAccess gives the
	// admin role for every rbac.KnownResourceTypes, agent_tool included)
	// already satisfies this via the existing HasAccessLevel cascade, no
	// extra code needed.
	repo := agenttool.NewRepository(setupTestDB(t))
	seedTool(t, repo, "read_file", agenttool.VisibilityPrivate)
	seedTool(t, repo, "write_file", agenttool.VisibilityPrivate)

	admin := agenttool.NewService(repo, allowAllLevels)
	items, total, err := admin.ListForCaller(context.Background(), 1, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListForCaller failed: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected a wildcard grant to reach both private tools, got total=%d len=%d", total, len(items))
	}
}

func TestUpdate_ChangesVisibility(t *testing.T) {
	repo := agenttool.NewRepository(setupTestDB(t))
	tool := seedTool(t, repo, "read_file", agenttool.VisibilityPublic)

	service := agenttool.NewService(repo, allowAllLevels)
	updated, err := service.Update(context.Background(), tool.ID, agenttool.UpdateToolRequest{Visibility: agenttool.VisibilityPrivate})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Visibility != agenttool.VisibilityPrivate {
		t.Fatalf("expected Visibility=private, got %v", updated.Visibility)
	}

	outsider := agenttool.NewService(repo, denyAllLevels)
	_, total, _ := outsider.ListForCaller(context.Background(), 999, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 10})
	if total != 0 {
		t.Fatalf("expected the tool to disappear for an outsider once flipped to private, got total=%d", total)
	}
}
