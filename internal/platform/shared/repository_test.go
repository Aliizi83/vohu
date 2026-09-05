package shared_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type repoTestWidget struct {
	shared.BaseModel
	Name string
}

func (repoTestWidget) TableName() string { return "repo_test_widgets" }

func setupRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&repoTestWidget{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestGenericRepository_CRUDLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := shared.NewGenericRepository[repoTestWidget](setupRepoTestDB(t))

	widget := &repoTestWidget{Name: "gadget"}
	if err := repo.Create(ctx, widget); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if widget.ID == 0 {
		t.Fatal("expected Create to populate an ID")
	}

	found, err := repo.FindByID(ctx, widget.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if found.Name != "gadget" {
		t.Fatalf("expected name 'gadget', got %q", found.Name)
	}

	found.Name = "renamed-gadget"
	if err := repo.Update(ctx, found); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	reloaded, err := repo.FindByID(ctx, widget.ID)
	if err != nil {
		t.Fatalf("FindByID after update failed: %v", err)
	}
	if reloaded.Name != "renamed-gadget" {
		t.Fatalf("expected updated name to persist, got %q", reloaded.Name)
	}

	if err := repo.Delete(ctx, widget.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = repo.FindByID(ctx, widget.ID)
	if !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("expected shared.ErrNotFound after delete, got %v", err)
	}
}

func TestGenericRepository_FindByID_NotFound(t *testing.T) {
	repo := shared.NewGenericRepository[repoTestWidget](setupRepoTestDB(t))

	_, err := repo.FindByID(context.Background(), 9999)
	if !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("expected shared.ErrNotFound, got %v", err)
	}
}

func TestGenericRepository_Delete_NotFound(t *testing.T) {
	repo := shared.NewGenericRepository[repoTestWidget](setupRepoTestDB(t))

	err := repo.Delete(context.Background(), 9999)
	if !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("expected shared.ErrNotFound for deleting a nonexistent row, got %v", err)
	}
}

func TestGenericRepository_List_PaginatesAndCounts(t *testing.T) {
	ctx := context.Background()
	db := setupRepoTestDB(t)
	repo := shared.NewGenericRepository[repoTestWidget](db)

	for i := range 15 {
		if err := repo.Create(ctx, &repoTestWidget{Name: "widget"}); err != nil {
			t.Fatalf("seed Create %d failed: %v", i, err)
		}
	}

	items, total, err := repo.List(ctx, shared.DynamicFilter{}, shared.Pagination{PageNumber: 2, PageSize: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if total != 15 {
		t.Fatalf("expected total=15, got %d", total)
	}
	if len(items) != 5 {
		t.Fatalf("expected 5 items on page 2 of 10, got %d", len(items))
	}
}
