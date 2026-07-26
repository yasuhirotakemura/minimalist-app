package item_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
)

func TestNewListCriteria_既定値(t *testing.T) {
	criteria, err := domainitem.NewListCriteria(domainitem.ListCriteriaInput{})
	if err != nil {
		t.Fatalf("NewListCriteria returned error: %v", err)
	}

	if criteria.SortKey != domainitem.DefaultSortKey {
		t.Errorf("SortKey = %q, want %q", criteria.SortKey, domainitem.DefaultSortKey)
	}
	if !criteria.Descending {
		t.Error("Descending = false, want true")
	}
	if criteria.Limit != domainitem.DefaultListLimit {
		t.Errorf("Limit = %d, want %d", criteria.Limit, domainitem.DefaultListLimit)
	}
	if criteria.Offset != 0 {
		t.Errorf("Offset = %d, want 0", criteria.Offset)
	}
	if criteria.IncludeArchived {
		t.Error("IncludeArchived = true, want false")
	}
}

func TestNewListCriteria_全条件を受け取る(t *testing.T) {
	categoryPublicID := uuid.New()
	tagPublicID := uuid.New()

	criteria, err := domainitem.NewListCriteria(domainitem.ListCriteriaInput{
		Keyword:            "  傘  ",
		CategoryPublicID:   &categoryPublicID,
		TagPublicID:        &tagPublicID,
		NecessityLevelCode: "essential",
		UsageFrequencyCode: "monthly",
		MobilityClassCode:  "daily_bag",
		IncludeArchived:    true,
		SortKeyName:        "name",
		Order:              "asc",
		Limit:              pointerTo(int32(10)),
		Offset:             pointerTo(int32(20)),
	})
	if err != nil {
		t.Fatalf("NewListCriteria returned error: %v", err)
	}

	if criteria.Keyword != "傘" {
		t.Errorf("Keyword = %q, want 傘", criteria.Keyword)
	}
	if criteria.CategoryPublicID == nil || *criteria.CategoryPublicID != categoryPublicID {
		t.Errorf("CategoryPublicID = %v, want %v", criteria.CategoryPublicID, categoryPublicID)
	}
	if criteria.TagPublicID == nil || *criteria.TagPublicID != tagPublicID {
		t.Errorf("TagPublicID = %v, want %v", criteria.TagPublicID, tagPublicID)
	}
	if criteria.NecessityLevel == nil ||
		*criteria.NecessityLevel != domainitem.NecessityLevelEssential {
		t.Errorf("NecessityLevel = %v, want essential", criteria.NecessityLevel)
	}
	if criteria.SortKey != domainitem.SortKeyName {
		t.Errorf("SortKey = %q, want name", criteria.SortKey)
	}
	if criteria.Descending {
		t.Error("Descending = true, want false")
	}
	if criteria.Limit != 10 || criteria.Offset != 20 {
		t.Errorf("Limit/Offset = %d/%d, want 10/20", criteria.Limit, criteria.Offset)
	}
	if !criteria.IncludeArchived {
		t.Error("IncludeArchived = false, want true")
	}
}

func TestNewListCriteria_異常系(t *testing.T) {
	testCases := map[string]struct {
		input     domainitem.ListCriteriaInput
		wantField string
	}{
		"keywordが上限超過": {
			input: domainitem.ListCriteriaInput{
				Keyword: strings.Repeat("あ", domainitem.MaxKeywordLength+1),
			},
			wantField: "keyword",
		},
		"sortが不正": {
			input:     domainitem.ListCriteriaInput{SortKeyName: "score"},
			wantField: "sort",
		},
		"orderが不正": {
			input:     domainitem.ListCriteriaInput{Order: "descending"},
			wantField: "order",
		},
		"limitが0": {
			input:     domainitem.ListCriteriaInput{Limit: pointerTo(int32(0))},
			wantField: "limit",
		},
		"limitが上限超過": {
			input: domainitem.ListCriteriaInput{
				Limit: pointerTo(domainitem.MaxListLimit + 1),
			},
			wantField: "limit",
		},
		"offsetが負": {
			input:     domainitem.ListCriteriaInput{Offset: pointerTo(int32(-1))},
			wantField: "offset",
		},
		"必要度が不正": {
			input:     domainitem.ListCriteriaInput{NecessityLevelCode: "very_important"},
			wantField: "necessityLevelCode",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			if _, err := domainitem.NewListCriteria(testCase.input); err == nil {
				t.Fatal("NewListCriteria returned nil error, want error")
			} else {
				assertFieldError(t, err, testCase.wantField)
			}
		})
	}
}

func TestNewListCriteria_境界値(t *testing.T) {
	testCases := map[string]domainitem.ListCriteriaInput{
		"limitが1": {Limit: pointerTo(int32(1))},
		"limitが上限": {
			Limit: pointerTo(domainitem.MaxListLimit),
		},
		"offsetが0": {Offset: pointerTo(int32(0))},
		"keywordが上限": {
			Keyword: strings.Repeat("あ", domainitem.MaxKeywordLength),
		},
	}

	for name, input := range testCases {
		t.Run(name, func(t *testing.T) {
			if _, err := domainitem.NewListCriteria(input); err != nil {
				t.Fatalf("NewListCriteria returned error: %v", err)
			}
		})
	}
}

func TestNewPageCriteria(t *testing.T) {
	page, err := domainitem.NewPageCriteria(nil, nil)
	if err != nil {
		t.Fatalf("NewPageCriteria returned error: %v", err)
	}
	if page.Limit != domainitem.DefaultListLimit || page.Offset != 0 {
		t.Errorf("page = %+v, want limit %d offset 0", page, domainitem.DefaultListLimit)
	}

	if _, err := domainitem.NewPageCriteria(pointerTo(int32(0)), nil); err == nil {
		t.Fatal("NewPageCriteria(limit=0) returned nil error, want error")
	}
	if _, err := domainitem.NewPageCriteria(nil, pointerTo(int32(-1))); err == nil {
		t.Fatal("NewPageCriteria(offset=-1) returned nil error, want error")
	}
}
