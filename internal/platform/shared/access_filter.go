package shared

import "context"

// FilterAndPaginate applies a per-row AccessLevelCheck to candidates (the
// caller should fetch these with a generously large limit rather than the
// real page size — a few hundred/thousand rows, not the true page size)
// and then paginates the *filtered* result in memory, so page's reported
// total matches what the caller can actually see rather than the
// pre-filter row count.
//
// Used by every module's ListForCaller-style method once resource-level
// (not just a flat "can see everything" permission) visibility applies.
// Translating the full resolver (direct grant, wildcard grant, role
// grant, role-cascade) into one SQL query per list isn't worth the
// complexity at this project's current scale — a per-row check against a
// generously-capped candidate set is simpler and provably correct; the
// candidate cap is the tradeoff to revisit if a resource type ever grows
// past a few thousand rows.
func FilterAndPaginate[T any](
	ctx context.Context,
	candidates []T,
	getID func(T) uint,
	hasAccessLevel AccessLevelCheck,
	userID uint,
	resourceType string,
	level string,
	page Pagination,
) ([]T, int64, error) {
	filtered := make([]T, 0, len(candidates))
	for _, item := range candidates {
		ok, err := hasAccessLevel(ctx, userID, resourceType, getID(item), level)
		if err != nil {
			return nil, 0, err
		}
		if ok {
			filtered = append(filtered, item)
		}
	}

	total := int64(len(filtered))
	start := page.Offset()
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + page.Limit()
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[start:end], total, nil
}
