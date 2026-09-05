package shared

import "math"

// Pagination mirrors sample-golang-project's PaginationInput. Its
// GetPageSize()/GetPageNumber() mutated the struct as a side effect of
// reading it — these are plain computed methods instead, same defaulting
// behavior (page 1, size 10) without the mutation.
type Pagination struct {
	PageNumber int `json:"pageNumber"`
	PageSize   int `json:"pageSize"`
}

func (p Pagination) pageNumber() int {
	if p.PageNumber < 1 {
		return 1
	}
	return p.PageNumber
}

func (p Pagination) pageSize() int {
	if p.PageSize < 1 {
		return 10
	}
	return p.PageSize
}

func (p Pagination) Offset() int {
	return (p.pageNumber() - 1) * p.pageSize()
}

func (p Pagination) Limit() int {
	return p.pageSize()
}

type PagedList[T any] struct {
	PageNumber      int   `json:"pageNumber"`
	PageSize        int   `json:"pageSize"`
	TotalRows       int64 `json:"totalRows"`
	TotalPages      int   `json:"totalPages"`
	HasPreviousPage bool  `json:"hasPreviousPage"`
	HasNextPage     bool  `json:"hasNextPage"`
	Items           []T   `json:"items"`
}

func NewPagedList[T any](items []T, total int64, page Pagination) PagedList[T] {
	pageSize := page.pageSize()
	pageNumber := page.pageNumber()
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	return PagedList[T]{
		PageNumber:      pageNumber,
		PageSize:        pageSize,
		TotalRows:       total,
		TotalPages:      totalPages,
		HasPreviousPage: pageNumber > 1,
		HasNextPage:     pageNumber < totalPages,
		Items:           items,
	}
}
