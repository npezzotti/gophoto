package pagination

import (
	"net/http/httptest"
	"slices"
	"testing"
)

func TestNewPagination(t *testing.T) {
	tests := []struct {
		name  string
		page  int
		limit int
		total int
	}{
		{
			name:  "valid pagination",
			page:  2,
			limit: 10,
			total: 50,
		},
		{
			name:  "zero page and limit uses defaults",
			page:  0,
			limit: 0,
			total: 50,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewPagination(tt.page, tt.limit, tt.total)
			if tt.limit > 0 && got.Limit != tt.limit {
				t.Errorf("got %d items per page, want %d", got.Limit, tt.limit)
			}
			if tt.page > 0 && got.Page != tt.page {
				t.Errorf("got %d current page, want %d", got.Page, tt.page)
			}
			if tt.page < 1 && got.Page != DefaultPage {
				t.Errorf("got %d current page, want default %d", got.Page, DefaultPage)
			}
			if tt.limit < 1 && got.Limit != DefaultLimit {
				t.Errorf("got %d items per page, want default %d", got.Limit, DefaultLimit)
			}
		})
	}
}

func TestNewPagination_TotalPages(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		limit     int
		wantPages int
	}{
		{
			name:      "total not divisible by limit rounds up",
			total:     25,
			limit:     12,
			wantPages: 3,
		},
		{
			name:      "total divisible by limit does not round up",
			total:     24,
			limit:     12,
			wantPages: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewPagination(1, tt.limit, tt.total)

			if got.TotalPages != tt.wantPages {
				t.Errorf("got %d total pages, want %d", got.TotalPages, tt.wantPages)
			}
		})
	}
}

func TestNewPaginationFromRequest(t *testing.T) {
	tcases := []struct {
		name  string
		query string
		page  int
		limit int
		total int
	}{
		{
			name:  "valid query parameters",
			query: "/?page=3&limit=15",
			page:  3,
			limit: 15,
		},
		{
			name:  "invalid query parameters",
			query: "/?page=abc&limit=xyz",
			page:  DefaultPage,
			limit: DefaultLimit,
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.query, nil)
			pagination := NewPaginationFromRequest(req)

			if pagination.Page != tt.page {
				t.Errorf("got %d current page, want %d", pagination.Page, tt.page)
			}
			if pagination.Limit != tt.limit {
				t.Errorf("got %d items per page, want %d", pagination.Limit, tt.limit)
			}
		})
	}
}

func Test_Pagination_SetTotal(t *testing.T) {
	tcases := []struct {
		name      string
		total     int
		limit     int
		wantPages int
	}{
		{
			name:      "set total updates total pages",
			total:     25,
			limit:     12,
			wantPages: 3,
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPagination(1, tt.limit, 0)
			p.SetTotal(tt.total)

			if p.TotalPages != tt.wantPages {
				t.Errorf("got %d total pages, want %d", p.TotalPages, tt.wantPages)
			}
		})
	}
}

func Test_parseInt(t *testing.T) {
	tests := []struct {
		name       string
		str        string
		defaultVal int
		want       int
	}{
		{
			name:       "empty string returns default",
			str:        "",
			defaultVal: 5,
			want:       5,
		},
		{
			name:       "valid integer string",
			str:        "10",
			defaultVal: 5,
			want:       10,
		},
		{
			name:       "invalid integer string returns default",
			str:        "abc",
			defaultVal: 5,
			want:       5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInt(tt.str, tt.defaultVal)

			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPagination_AdjacentPages(t *testing.T) {
	tcases := []struct {
		name      string
		page      int
		total     int
		limit     int
		wantPages []int
	}{
		{
			name:      "total pages <= 10, current page in middle",
			page:      5,
			total:     100,
			limit:     10,
			wantPages: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
		{
			name:      "total pages > 10, current page near start",
			page:      2,
			total:     120,
			limit:     10,
			wantPages: []int{1, 2, 3, 4, 5, 6, 7, -1, 11, 12},
		},
		{
			name:      "total pages > 10, current page in middle",
			page:      6,
			total:     120,
			limit:     10,
			wantPages: []int{1, 2, -1, 4, 5, 6, 7, -1, 11, 12},
		},
		{
			name:      "total pages > 10, current page near end",
			page:      11,
			total:     120,
			limit:     10,
			wantPages: []int{1, 2, -1, 6, 7, 8, 9, 10, 11, 12},
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPagination(tt.page, tt.limit, tt.total)
			got := p.AdjacentPages()

			if !slices.Equal(got, tt.wantPages) {
				t.Errorf("got %v, want %v", got, tt.wantPages)
			}
		})
	}
}

func TestPagination_HasNext(t *testing.T) {
	tcases := []struct {
		name  string
		page  int
		total int
		limit int
		want  bool
	}{
		{
			name:  "has next page",
			page:  1,
			total: 20,
			limit: 10,
			want:  true,
		},
		{
			name:  "no next page",
			page:  2,
			total: 20,
			limit: 10,
			want:  false,
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPagination(tt.page, tt.limit, tt.total)
			got := p.HasNext()
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPagination_HasPrev(t *testing.T) {
	tcases := []struct {
		name  string
		page  int
		total int
		limit int
		want  bool
	}{
		{
			name:  "has previous page",
			page:  2,
			total: 20,
			limit: 10,
			want:  true,
		},
		{
			name:  "no previous page",
			page:  1,
			total: 20,
			limit: 10,
			want:  false,
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPagination(tt.page, tt.limit, tt.total)
			got := p.HasPrev()
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPagination_NextPage(t *testing.T) {
	tcases := []struct {
		name  string
		page  int
		total int
		limit int
		want  int
	}{
		{
			name:  "has next page",
			page:  1,
			total: 20,
			limit: 10,
			want:  2,
		},
		{
			name:  "no next page",
			page:  2,
			total: 20,
			limit: 10,
			want:  2,
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPagination(tt.page, tt.limit, tt.total)
			got := p.NextPage()
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPagination_PrevPage(t *testing.T) {
	tcases := []struct {
		name  string
		page  int
		total int
		limit int
		want  int
	}{
		{
			name:  "has previous page",
			page:  2,
			total: 20,
			limit: 10,
			want:  1,
		},
		{
			name:  "no previous page",
			page:  1,
			total: 20,
			limit: 10,
			want:  1,
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPagination(tt.page, tt.limit, tt.total)
			got := p.PrevPage()
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPagination_Offset(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		limit    int
		expected int
	}{
		{
			name:     "first page",
			page:     1,
			limit:    10,
			expected: 0,
		},
		{
			name:     "second page",
			page:     2,
			limit:    10,
			expected: 10,
		},
		{
			name:     "third page",
			page:     3,
			limit:    10,
			expected: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPagination(tt.page, tt.limit, 100)
			got := p.Offset()
			if got != tt.expected {
				t.Errorf("got %d, want %d", got, tt.expected)
			}
		})
	}
}
