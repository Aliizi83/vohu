package shared_test

// Uses lessThan/greaterThan/inRange (portable SQL) rather than the
// ILIKE-based operators (contains/equals/...), which are Postgres syntax —
// SQLite doesn't have ILIKE. The safety property under test (values are
// always bind parameters, never interpolated into the query) is the same
// mechanism for every operator; buildCondition in filter.go always returns
// "? "-parameterized args regardless of which operator branch runs. The
// exact `' OR '1'='1` injection payload against the ILIKE path was
// verified live against real Postgres this session (see the commit that
// added filter.go) rather than duplicated here as an automated test.

import (
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type filterTestAuthor struct {
	shared.BaseModel
	Name string
	Age  int
}

func (filterTestAuthor) TableName() string { return "filter_test_authors" }

type filterTestBook struct {
	shared.BaseModel
	Title    string
	AuthorID uint
	Author   filterTestAuthor
}

func (filterTestBook) TableName() string { return "filter_test_books" }

func setupFilterTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}

	if err := db.AutoMigrate(&filterTestAuthor{}, &filterTestBook{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	authors := []filterTestAuthor{
		{Name: "Ada", Age: 36},
		{Name: "Grace", Age: 85},
	}
	if err := db.Create(&authors).Error; err != nil {
		t.Fatalf("failed to seed authors: %v", err)
	}

	books := []filterTestBook{
		{Title: "Book One", AuthorID: authors[0].ID},
		{Title: "Book Two", AuthorID: authors[1].ID},
	}
	if err := db.Create(&books).Error; err != nil {
		t.Fatalf("failed to seed books: %v", err)
	}

	return db
}

func TestApplyDynamicFilter_NumericOperators(t *testing.T) {
	db := setupFilterTestDB(t)

	query, err := shared.ApplyDynamicFilter[filterTestAuthor](db, shared.DynamicFilter{
		Filters: map[string]shared.FieldFilter{
			"Age": {Type: shared.OpGreaterThan, From: "50"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var results []filterTestAuthor
	if err := query.Find(&results).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(results) != 1 || results[0].Name != "Grace" {
		t.Fatalf("expected only Grace (age 85), got %+v", results)
	}
}

func TestApplyDynamicFilter_UnknownFieldIsSkippedNotErrored(t *testing.T) {
	db := setupFilterTestDB(t)

	query, err := shared.ApplyDynamicFilter[filterTestAuthor](db, shared.DynamicFilter{
		Filters: map[string]shared.FieldFilter{
			"NotARealField; DROP TABLE filter_test_authors; --": {Type: shared.OpEquals, From: "x"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var results []filterTestAuthor
	if err := query.Find(&results).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected an unrecognized filter key to be a no-op (both rows returned), got %d rows", len(results))
	}
}

// TestApplyDynamicFilter_ValueIsNeverExecutedAsSQL is the portable version
// of the injection check: a value shaped like a SQL statement is used as a
// literal comparison value, not executed. If the old
// fmt.Sprintf-into-.Where(rawString) approach were still in place, a
// payload like this would either error out (malformed SQL) or, with the
// right shape, alter query semantics. Here it can only ever be a bind
// parameter.
func TestApplyDynamicFilter_ValueIsNeverExecutedAsSQL(t *testing.T) {
	db := setupFilterTestDB(t)

	query, err := shared.ApplyDynamicFilter[filterTestAuthor](db, shared.DynamicFilter{
		Filters: map[string]shared.FieldFilter{
			"Name": {Type: shared.OpGreaterThan, From: "'; DROP TABLE filter_test_authors; --"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var results []filterTestAuthor
	if err := query.Find(&results).Error; err != nil {
		t.Fatalf("query failed — a real injection would likely break the SQL syntax entirely: %v", err)
	}

	// The table must still exist and be queryable afterward.
	var count int64
	if err := db.Model(&filterTestAuthor{}).Count(&count).Error; err != nil {
		t.Fatalf("filter_test_authors table appears to have been affected: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected the table to be untouched (2 rows), got %d", count)
	}
}

func TestApplyDynamicFilter_OneLevelNestedJoin(t *testing.T) {
	db := setupFilterTestDB(t)

	query, err := shared.ApplyDynamicFilter[filterTestBook](db, shared.DynamicFilter{
		Filters: map[string]shared.FieldFilter{
			"Author.Age": {Type: shared.OpGreaterThan, From: "50"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var results []filterTestBook
	if err := query.Find(&results).Error; err != nil {
		t.Fatalf("joined query failed: %v", err)
	}

	if len(results) != 1 || results[0].Title != "Book Two" {
		t.Fatalf("expected only Book Two (author Grace, age 85), got %+v", results)
	}
}

func TestApplySort(t *testing.T) {
	db := setupFilterTestDB(t)

	query, err := shared.ApplySort[filterTestAuthor](db, []shared.SortField{
		{ColumnID: "Age", Direction: "desc"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var results []filterTestAuthor
	if err := query.Find(&results).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(results) != 2 || results[0].Name != "Grace" || results[1].Name != "Ada" {
		t.Fatalf("expected descending age order [Grace, Ada], got %+v", results)
	}
}

func TestApplySort_InvalidDirectionIsIgnored(t *testing.T) {
	db := setupFilterTestDB(t)

	query, err := shared.ApplySort[filterTestAuthor](db, []shared.SortField{
		{ColumnID: "Age", Direction: "sideways; DROP TABLE filter_test_authors"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var results []filterTestAuthor
	if err := query.Find(&results).Error; err != nil {
		t.Fatalf("expected an invalid direction to just be ignored, got a query error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected both rows back, got %d", len(results))
	}
}
