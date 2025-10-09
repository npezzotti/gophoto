package pagination

import (
	"net/http"
	"strconv"
)

var (
	DefaultPage  = 1
	DefaultLimit = 10
)

type Pagination struct {
	Limit      int
	Page       int
	Total      int
	TotalPages int
}

func NewPagination(page, limit, total int) *Pagination {
	if page < 1 {
		page = DefaultPage
	}

	if limit < 1 {
		limit = DefaultLimit
	}

	totalPages := (total + limit - 1) / limit

	paginator := &Pagination{
		Limit:      limit,
		Page:       page,
		Total:      total,
		TotalPages: totalPages,
	}

	return paginator
}

func NewPaginationFromRequest(r *http.Request, total int) *Pagination {
	page := parseInt(r.URL.Query().Get("page"), DefaultPage)
	limit := parseInt(r.URL.Query().Get("limit"), DefaultLimit)

	return NewPagination(page, limit, total)
}

func parseInt(str string, defaultVal int) int {
	if str == "" {
		return defaultVal
	}

	if res, err := strconv.Atoi(str); err == nil {
		return res
	}

	return defaultVal
}

// AdjacentPages returns a slice of integers representing
// adjacent pages to be used to render a pagination
// block in a template.
func (p *Pagination) AdjacentPages() []int {
	var pages []int
	const adjacents = 2

	if p.TotalPages <= 10 {
		for i := 1; i <= p.TotalPages; i++ {
			pages = append(pages, i)
		}
	} else if p.TotalPages >= 10 {
		if p.Page <= 4 {
			for i := 1; i < 8; i++ {
				pages = append(pages, i)
			}
			pages = append(pages, -1, p.TotalPages-1, p.TotalPages)
		} else if p.Page > 4 && p.Page < p.TotalPages-4 {
			pages = append(pages, 1, 2, -1)

			for i := p.Page - adjacents; i < p.Page+adjacents; i++ {
				pages = append(pages, i)
			}

			pages = append(pages, -1, p.TotalPages-1, p.TotalPages)
		} else {
			pages = append(pages, 1, 2, -1)

			for i := p.TotalPages - 6; i <= p.TotalPages; i++ {
				pages = append(pages, i)
			}
		}
	}

	return pages
}

func (p *Pagination) HasNext() bool {
	return p.Page < p.TotalPages
}

func (p *Pagination) HasPrev() bool {
	return p.Page > 1
}

func (p *Pagination) NextPage() int {
	if p.HasNext() {
		return p.Page + 1
	}

	return p.Page
}

func (p *Pagination) PrevPage() int {
	if p.HasPrev() {
		return p.Page - 1
	}

	return p.Page
}

func (p *Pagination) Offset() int {
	return (p.Page - 1) * p.Limit
}
