package storage_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
)

func TestNewListCriteriaAppliesDefaults(t *testing.T) {
	criteria, err := domainstorage.NewListCriteria(domainstorage.ListCriteriaInput{})
	if err != nil {
		t.Fatalf("NewListCriteria returned error: %v", err)
	}

	if criteria.SortKey != domainstorage.SortKeySortOrder {
		t.Errorf("SortKey = %s, want sortOrder", criteria.SortKey)
	}
	if criteria.Descending {
		t.Error("Descending = true, want false")
	}
	if criteria.Limit != domainstorage.DefaultListLimit {
		t.Errorf("Limit = %d, want %d", criteria.Limit, domainstorage.DefaultListLimit)
	}
	if criteria.IncludeArchived {
		t.Error("IncludeArchived = true, want false")
	}
}

func TestNewListCriteriaParsesCodes(t *testing.T) {
	parentPublicID := publicIDOf("01")

	criteria, err := domainstorage.NewListCriteria(domainstorage.ListCriteriaInput{
		Keyword:           "  リュック  ",
		StorageTypeCode:   "bag",
		MobilityClassCode: "daily_bag",
		ParentPublicID:    &parentPublicID,
		IncludeArchived:   true,
		SortKeyName:       "updatedAt",
		Order:             "desc",
		Limit:             pointerTo(int32(10)),
		Offset:            pointerTo(int32(20)),
	})
	if err != nil {
		t.Fatalf("NewListCriteria returned error: %v", err)
	}

	if criteria.Keyword != "リュック" {
		t.Errorf("Keyword = %q, want リュック", criteria.Keyword)
	}
	if criteria.StorageType == nil || *criteria.StorageType != domainstorage.StorageTypeBag {
		t.Errorf("StorageType = %v, want bag", criteria.StorageType)
	}
	if criteria.MobilityClass == nil ||
		*criteria.MobilityClass != domainitem.MobilityClassDailyBag {
		t.Errorf("MobilityClass = %v, want daily_bag", criteria.MobilityClass)
	}
	if !criteria.Descending {
		t.Error("Descending = false, want true")
	}
	if criteria.Limit != 10 || criteria.Offset != 20 {
		t.Errorf("Limit/Offset = %d/%d, want 10/20", criteria.Limit, criteria.Offset)
	}
}

func TestNewListCriteriaRejectsInvalidInput(t *testing.T) {
	parentPublicID := uuid.New()

	testCases := map[string]struct {
		input domainstorage.ListCriteriaInput
		field string
	}{
		"rootOnlyと親指定の同時指定": {
			input: domainstorage.ListCriteriaInput{
				RootOnly: true, ParentPublicID: &parentPublicID,
			},
			field: "rootOnly",
		},
		"長すぎるkeyword": {
			input: domainstorage.ListCriteriaInput{Keyword: strings.Repeat("あ", 101)},
			field: "keyword",
		},
		"未定義のsort key": {
			input: domainstorage.ListCriteriaInput{SortKeyName: "totalWeightGram"},
			field: "sort",
		},
		"未定義のorder": {
			input: domainstorage.ListCriteriaInput{Order: "random"},
			field: "order",
		},
		"上限を超えるlimit": {
			input: domainstorage.ListCriteriaInput{Limit: pointerTo(int32(101))},
			field: "limit",
		},
		"負のoffset": {
			input: domainstorage.ListCriteriaInput{Offset: pointerTo(int32(-1))},
			field: "offset",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			_, err := domainstorage.NewListCriteria(testCase.input)
			if err == nil {
				t.Fatal("NewListCriteria returned nil error, want error")
			}
			assertFieldError(t, err, testCase.field)
		})
	}
}

func TestNewStorageType(t *testing.T) {
	storageType, err := domainstorage.NewStorageType(" pouch ")
	if err != nil {
		t.Fatalf("NewStorageType returned error: %v", err)
	}
	if storageType != domainstorage.StorageTypePouch {
		t.Errorf("StorageType = %s, want pouch", storageType)
	}
	if storageType.Label() == "" {
		t.Error("Label is empty")
	}

	if _, err := domainstorage.NewStorageType(""); err == nil {
		t.Fatal("NewStorageType(\"\") returned nil error, want error")
	}
}

func TestNewHierarchyDepth(t *testing.T) {
	for _, depth := range []int32{1, 2, 3} {
		if _, err := domainstorage.NewHierarchyDepth(depth); err != nil {
			t.Fatalf("depth %d returned error: %v", depth, err)
		}
	}
	if _, err := domainstorage.NewHierarchyDepth(4); err == nil {
		t.Fatal("depth 4 returned nil error, want ErrStorageHierarchyTooDeep")
	}
	if _, err := domainstorage.NewHierarchyDepth(0); err == nil {
		t.Fatal("depth 0 returned nil error, want error")
	}
}
