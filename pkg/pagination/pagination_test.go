package pagination

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name        string
		page, limit int
		offset      int
		wantLimit   int
		wantOffset  int
	}{
		{"zero value uses defaults", 0, 0, 0, DefaultLimit, 0},
		{"page converts to offset", 3, 10, 99, 10, 20},
		{"limit is capped", 0, 500, 0, MaxLimit, 0},
		{"negative limit falls back", 0, -5, 7, DefaultLimit, 7},
		{"offset passes through", 0, 25, 50, 25, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, offset := Normalize(tt.page, tt.limit, tt.offset)
			if limit != tt.wantLimit || offset != tt.wantOffset {
				t.Fatalf("got limit=%d offset=%d, want limit=%d offset=%d", limit, offset, tt.wantLimit, tt.wantOffset)
			}
		})
	}
}

func TestNewMeta(t *testing.T) {
	tests := []struct {
		name                string
		total               int64
		limit, offset       int
		wantPage, wantPages int
		wantNext, wantPrev  bool
	}{
		{"first page", 50, 20, 0, 1, 3, true, false},
		{"last page", 50, 20, 40, 3, 3, false, true},
		{"exact multiple", 40, 20, 20, 2, 2, false, true},
		{"empty catalog", 0, 20, 0, 1, 0, false, false},
		{"unaligned offset", 50, 20, 10, 1, 3, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMeta(tt.total, tt.limit, tt.offset)
			if m.Page != tt.wantPage || m.TotalPages != tt.wantPages {
				t.Fatalf("page=%d total_pages=%d, want %d/%d", m.Page, m.TotalPages, tt.wantPage, tt.wantPages)
			}
			if m.HasNext != tt.wantNext || m.HasPrev != tt.wantPrev {
				t.Fatalf("has_next=%v has_prev=%v, want %v/%v", m.HasNext, m.HasPrev, tt.wantNext, tt.wantPrev)
			}
			if m.Total != tt.total || m.Limit != tt.limit || m.Offset != tt.offset {
				t.Fatalf("counters not copied: %+v", m)
			}
		})
	}
}
