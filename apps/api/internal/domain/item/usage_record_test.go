package item_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
)

func TestNewUsageRecord_正常系(t *testing.T) {
	usedAt := testNow.Add(-time.Hour)

	record, err := domainitem.NewUsageRecord(
		testPublicID, testUserID, domainitem.ItemID(3), usedAt, 2, pointerTo("  通勤  "), testNow)
	if err != nil {
		t.Fatalf("NewUsageRecord returned error: %v", err)
	}

	if !record.UsedAt().Equal(usedAt) {
		t.Errorf("UsedAt = %v, want %v", record.UsedAt(), usedAt)
	}
	if record.Quantity() != 2 {
		t.Errorf("Quantity = %d, want 2", record.Quantity())
	}
	if record.Note() == nil || *record.Note() != "通勤" {
		t.Errorf("Note = %v, want 通勤", record.Note())
	}
	if !record.CreatedAt().Equal(testNow) {
		t.Errorf("CreatedAt = %v, want %v", record.CreatedAt(), testNow)
	}
}

func TestNewUsageRecord_既定値(t *testing.T) {
	record, err := domainitem.NewUsageRecord(
		testPublicID, testUserID, domainitem.ItemID(3), time.Time{}, 0, nil, testNow)
	if err != nil {
		t.Fatalf("NewUsageRecord returned error: %v", err)
	}

	if !record.UsedAt().Equal(testNow) {
		t.Errorf("UsedAt = %v, want %v (現在時刻)", record.UsedAt(), testNow)
	}
	if record.Quantity() != domainitem.DefaultUsageQuantity {
		t.Errorf("Quantity = %d, want %d", record.Quantity(), domainitem.DefaultUsageQuantity)
	}
	if record.Note() != nil {
		t.Errorf("Note = %v, want nil", *record.Note())
	}
}

func TestNewUsageRecord_異常系(t *testing.T) {
	testCases := map[string]struct {
		usedAt    time.Time
		quantity  int32
		note      *string
		wantField string
	}{
		"使用日時が未来": {
			usedAt:    testNow.Add(time.Minute),
			quantity:  1,
			wantField: "usedAt",
		},
		"数量が負": {
			usedAt:    testNow,
			quantity:  -1,
			wantField: "quantity",
		},
		"数量が上限超過": {
			usedAt:    testNow,
			quantity:  domainitem.MaxUsageQuantity + 1,
			wantField: "quantity",
		},
		"備考が上限超過": {
			usedAt:    testNow,
			quantity:  1,
			note:      pointerTo(strings.Repeat("あ", domainitem.MaxUsageNoteLength+1)),
			wantField: "note",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			_, err := domainitem.NewUsageRecord(
				testPublicID, testUserID, domainitem.ItemID(3),
				testCase.usedAt, testCase.quantity, testCase.note, testNow)
			if err == nil {
				t.Fatal("NewUsageRecord returned nil error, want error")
			}
			assertFieldError(t, err, testCase.wantField)
		})
	}
}

func TestNewUsageRecord_境界値(t *testing.T) {
	testCases := map[string]struct {
		usedAt   time.Time
		quantity int32
		note     *string
	}{
		"使用日時が現在時刻": {usedAt: testNow, quantity: 1},
		"数量が1":      {usedAt: testNow, quantity: domainitem.MinUsageQuantity},
		"数量が上限":     {usedAt: testNow, quantity: domainitem.MaxUsageQuantity},
		"備考が上限": {
			usedAt:   testNow,
			quantity: 1,
			note:     pointerTo(strings.Repeat("あ", domainitem.MaxUsageNoteLength)),
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			if _, err := domainitem.NewUsageRecord(
				testPublicID, testUserID, domainitem.ItemID(3),
				testCase.usedAt, testCase.quantity, testCase.note, testNow); err != nil {
				t.Fatalf("NewUsageRecord returned error: %v", err)
			}
		})
	}
}

func TestNewUsageRecord_publicIDが未設定(t *testing.T) {
	if _, err := domainitem.NewUsageRecord(
		uuid.Nil, testUserID, domainitem.ItemID(3), testNow, 1, nil, testNow); err == nil {
		t.Fatal("NewUsageRecord returned nil error, want error")
	}
}
