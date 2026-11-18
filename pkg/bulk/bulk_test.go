package bulk_test

import (
	"errors"
	"testing"

	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
)

func TestCheckCount(t *testing.T) {
	if err := bulk.CheckCount(0, 10); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("empty batch: expected VALIDATION_ERROR, got %v", err)
	}
	if err := bulk.CheckCount(3, 10); err != nil {
		t.Fatalf("under cap: %v", err)
	}
	if err := bulk.CheckCount(11, 10); apperr.CodeOf(err) != apperr.CodeBulkLimitExceeded {
		t.Fatalf("over cap: expected BULK_LIMIT_EXCEEDED, got %v", err)
	}
}

func TestParseIDs(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   []string
		code   apperr.Code
	}{
		{name: "repeated", values: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "comma", values: []string{"a,b"}, want: []string{"a", "b"}},
		{name: "mixed trims blanks", values: []string{" a , ", "", "b"}, want: []string{"a", "b"}},
		{name: "empty", values: nil, code: apperr.CodeValidation},
		{name: "duplicate", values: []string{"a", "a,b"}, code: apperr.CodeDuplicateItem},
		{name: "over cap", values: []string{"a", "b", "c"}, code: apperr.CodeBulkLimitExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bulk.ParseIDs(tt.values, 2)
			if tt.code != "" {
				if apperr.CodeOf(err) != tt.code {
					t.Fatalf("expected %s, got %v", tt.code, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("expected %v, got %v", tt.want, got)
				}
			}
		})
	}
}

func TestFindDuplicateAndLineKey(t *testing.T) {
	if dup, ok := bulk.FindDuplicate([]string{"a", "b", "a"}); !ok || dup != "a" {
		t.Fatalf("expected duplicate a, got %q %v", dup, ok)
	}
	if _, ok := bulk.FindDuplicate([]string{"a", "b"}); ok {
		t.Fatal("unexpected duplicate")
	}
	variant := "v1"
	if bulk.LineKey("p1", nil) == bulk.LineKey("p1", &variant) {
		t.Fatal("base and variant lines must differ")
	}
}

func TestResultsHelpers(t *testing.T) {
	variant := "v1"
	results := []bulk.BulkResult[string]{
		bulk.OK(0, "p1", "done"),
		bulk.Failed[string](1, "v1", apperr.NotFound("variant not found")),
	}
	results[0].ID = bulk.LineKey("p1", nil)
	results[1].ID = bulk.LineKey("p1", &variant)

	if results[0].Status != "ok" || results[0].Data == nil || *results[0].Data != "done" {
		t.Fatalf("bad ok result: %+v", results[0])
	}
	if results[1].Error == nil || results[1].Error.Code != apperr.CodeNotFound {
		t.Fatalf("bad failed result: %+v", results[1])
	}

	// Foreign errors must not leak their message.
	foreign := bulk.Failed[string](0, "x", errors.New("pg: secret detail"))
	if foreign.Error == nil || foreign.Error.Message != "internal server error" {
		t.Fatalf("foreign error leaked: %+v", foreign.Error)
	}

	summary := bulk.Summary(results)
	if summary.Total != 2 || summary.Succeeded != 1 || summary.Failed != 1 {
		t.Fatalf("bad summary: %+v", summary)
	}

	rolled := bulk.Rollback(results, 1, apperr.Conflict("boom"))
	for i, r := range rolled {
		if r.Status != "rolled_back" || r.Data != nil {
			t.Fatalf("item %d not rolled back: %+v", i, r)
		}
	}
	if rolled[0].Error != nil {
		t.Fatalf("only the failing item keeps an error: %+v", rolled[0])
	}
	if rolled[1].Error == nil || rolled[1].Error.Code != apperr.CodeConflict {
		t.Fatalf("failing item lost its error: %+v", rolled[1])
	}
	if s := bulk.Summary(rolled); s.Succeeded != 0 || s.Failed != 2 {
		t.Fatalf("rolled back must count as failed: %+v", s)
	}
}

func TestSetMaxItemsIgnoresNonPositive(t *testing.T) {
	original := bulk.MaxItems()
	t.Cleanup(func() { bulk.SetMaxItems(original) })

	bulk.SetMaxItems(7)
	if bulk.MaxItems() != 7 {
		t.Fatalf("expected 7, got %d", bulk.MaxItems())
	}
	bulk.SetMaxItems(0)
	if bulk.MaxItems() != 7 {
		t.Fatalf("zero must be ignored, got %d", bulk.MaxItems())
	}
}
