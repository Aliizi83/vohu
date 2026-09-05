package shared

import (
	"reflect"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type FilterOperator string

const (
	OpContains           FilterOperator = "contains"
	OpNotContains        FilterOperator = "notContains"
	OpStartsWith         FilterOperator = "startsWith"
	OpEndsWith           FilterOperator = "endsWith"
	OpEquals             FilterOperator = "equals"
	OpNotEquals          FilterOperator = "notEquals"
	OpLessThan           FilterOperator = "lessThan"
	OpLessThanOrEqual    FilterOperator = "lessThanOrEqual"
	OpGreaterThan        FilterOperator = "greaterThan"
	OpGreaterThanOrEqual FilterOperator = "greaterThanOrEqual"
	OpInRange            FilterOperator = "inRange"
)

type FieldFilter struct {
	Type FilterOperator `json:"type"`
	From string         `json:"from"`
	To   string         `json:"to"`
}

type SortField struct {
	ColumnID  string `json:"columnId"`
	Direction string `json:"sort"` // "asc" or "desc"
}

// DynamicFilter mirrors sample-golang-project's domain/filters.DynamicFilter
// shape exactly (same JSON tags), so this is a drop-in replacement for API
// consumers. What changed is entirely inside how it gets turned into SQL.
type DynamicFilter struct {
	Sorts   []SortField            `json:"sorts"`
	Filters map[string]FieldFilter `json:"filters"`
}

var namingStrategy = schema.NamingStrategy{}

// ApplyDynamicFilter adds a WHERE condition for each entry in
// filter.Filters. This replaces sample-golang-project's
// infra/presistence/db.GenerateDynamicFilter, which built the WHERE clause
// with fmt.Sprintf("%s ILIKE '%%%s%%'", column, filter.From) — the filter
// *value* went straight into the SQL string, an unfiltered client input
// like `' OR '1'='1` would inject. Here the value is always a bind
// parameter (`?`); the only thing that ever touches the query text is the
// column name, and that's resolved via reflection against T's own struct
// fields (or, one level deep, an association's fields) — a key that
// doesn't match a real field is silently skipped, never turned into SQL.
func ApplyDynamicFilter[T any](db *gorm.DB, filter DynamicFilter) (*gorm.DB, error) {
	t := reflect.TypeOf(*new(T))
	joined := make(map[string]bool)

	for key, f := range filter.Filters {
		column, joinRelation, ok := resolveColumn(t, key)
		if !ok {
			continue
		}

		if joinRelation != "" && !joined[joinRelation] {
			db = db.Joins(joinRelation)
			joined[joinRelation] = true
		}

		clause, args := buildCondition(column, f)
		if clause == "" {
			continue
		}

		db = db.Where(clause, args...)
	}

	return db, nil
}

// ApplySort adds ORDER BY clauses, same column-resolution rule as filters.
func ApplySort[T any](db *gorm.DB, sorts []SortField) (*gorm.DB, error) {
	t := reflect.TypeOf(*new(T))
	joined := make(map[string]bool)

	for _, s := range sorts {
		if s.Direction != "asc" && s.Direction != "desc" {
			continue
		}

		column, joinRelation, ok := resolveColumn(t, s.ColumnID)
		if !ok {
			continue
		}

		if joinRelation != "" && !joined[joinRelation] {
			db = db.Joins(joinRelation)
			joined[joinRelation] = true
		}

		db = db.Order(column + " " + s.Direction)
	}

	return db, nil
}

// resolveColumn turns a filter/sort key into a safe, server-controlled
// column reference. key is either "FieldName" (a direct field on T) or
// "Relation.FieldName" (one level of association, e.g. "Role.Name") — in
// the latter case joinRelation is the Go field name to pass to gorm's
// Joins(), and the returned column is qualified with that same field name
// (gorm's Joins("Author") aliases the joined table as "Author", the
// association's Go field name, not its TableName() — so the WHERE clause
// has to match that alias). Anything that doesn't resolve to a real
// field/association returns ok=false; the caller skips it rather than
// ever building SQL from it.
func resolveColumn(t reflect.Type, key string) (column string, joinRelation string, ok bool) {
	before, after, hasDot := strings.Cut(key, ".")

	if !hasDot {
		field, fieldOk := t.FieldByName(key)
		if !fieldOk {
			return "", "", false
		}
		return namingStrategy.ColumnName("", field.Name), "", true
	}

	relationField, relOk := t.FieldByName(before)
	if !relOk {
		return "", "", false
	}

	relationType := relationField.Type
	for relationType.Kind() == reflect.Ptr || relationType.Kind() == reflect.Slice {
		relationType = relationType.Elem()
	}
	if relationType.Kind() != reflect.Struct {
		return "", "", false
	}

	targetField, fieldOk := relationType.FieldByName(after)
	if !fieldOk {
		return "", "", false
	}

	// gorm's Joins(before) aliases the joined table as the association's Go
	// field name itself (before), not its TableName() — so the WHERE
	// clause must qualify with that same alias. before is exactly as safe
	// to use here as a bare column name: it only ever reaches this line
	// after t.FieldByName(before) succeeded above, so it can only be one
	// of T's own real field names, never attacker-controlled text.
	return before + "." + namingStrategy.ColumnName("", targetField.Name), before, true
}

func buildCondition(column string, f FieldFilter) (string, []any) {
	switch f.Type {
	case OpContains:
		return column + " ILIKE ?", []any{"%" + f.From + "%"}
	case OpNotContains:
		return column + " NOT ILIKE ?", []any{"%" + f.From + "%"}
	case OpStartsWith:
		return column + " ILIKE ?", []any{f.From + "%"}
	case OpEndsWith:
		return column + " ILIKE ?", []any{"%" + f.From}
	case OpEquals:
		return column + " ILIKE ?", []any{f.From}
	case OpNotEquals:
		return column + " NOT ILIKE ?", []any{f.From}
	case OpLessThan:
		return column + " < ?", []any{f.From}
	case OpLessThanOrEqual:
		return column + " <= ?", []any{f.From}
	case OpGreaterThan:
		return column + " > ?", []any{f.From}
	case OpGreaterThanOrEqual:
		return column + " >= ?", []any{f.From}
	case OpInRange:
		return column + " >= ? AND " + column + " <= ?", []any{f.From, f.To}
	default:
		return "", nil
	}
}
