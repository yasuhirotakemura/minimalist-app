package audit_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
)

var (
	testNow      = time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	testUserID   = domainauth.UserID(1)
	testPublicID = uuid.MustParse("018f8d0a-1c2b-7a3d-9e4f-5a6b7c8d9e0f")
)

func pointerTo[T any](value T) *T { return &value }

func TestNewAuditLog_正常系(t *testing.T) {
	targetPublicID := uuid.New()

	log, err := domainaudit.NewAuditLog(
		testPublicID,
		testUserID,
		domainaudit.ActionItemCreated,
		domainaudit.TargetTypeItem,
		&targetPublicID,
		domainaudit.Changes{"name": {From: nil, To: "折りたたみ傘"}},
		testNow,
	)
	if err != nil {
		t.Fatalf("NewAuditLog returned error: %v", err)
	}

	if log.Action() != domainaudit.ActionItemCreated {
		t.Errorf("Action = %q, want item_created", log.Action())
	}
	if log.TargetType() != domainaudit.TargetTypeItem {
		t.Errorf("TargetType = %q, want item", log.TargetType())
	}
	if log.TargetPublicID() == nil || *log.TargetPublicID() != targetPublicID {
		t.Errorf("TargetPublicID = %v, want %v", log.TargetPublicID(), targetPublicID)
	}
	if !log.CreatedAt().Equal(testNow) {
		t.Errorf("CreatedAt = %v, want %v", log.CreatedAt(), testNow)
	}
}

func TestNewAuditLog_changesがnilなら空objectとする(t *testing.T) {
	log, err := domainaudit.NewAuditLog(
		testPublicID, testUserID, domainaudit.ActionTagDeleted,
		domainaudit.TargetTypeTag, nil, nil, testNow)
	if err != nil {
		t.Fatalf("NewAuditLog returned error: %v", err)
	}
	if log.Changes() == nil {
		t.Fatal("Changes = nil, want empty Changes")
	}
	if len(log.Changes()) != 0 {
		t.Errorf("len(Changes) = %d, want 0", len(log.Changes()))
	}
}

func TestNewAuditLog_異常系(t *testing.T) {
	testCases := map[string]struct {
		publicID   uuid.UUID
		userID     domainauth.UserID
		action     domainaudit.ActionCode
		targetType domainaudit.TargetTypeCode
	}{
		"publicIDが未設定": {
			publicID: uuid.Nil, userID: testUserID,
			action: domainaudit.ActionItemCreated, targetType: domainaudit.TargetTypeItem,
		},
		"userIDが未設定": {
			publicID: testPublicID, userID: domainauth.UserID(0),
			action: domainaudit.ActionItemCreated, targetType: domainaudit.TargetTypeItem,
		},
		"actionが空": {
			publicID: testPublicID, userID: testUserID,
			action: "", targetType: domainaudit.TargetTypeItem,
		},
		"targetTypeが空": {
			publicID: testPublicID, userID: testUserID,
			action: domainaudit.ActionItemCreated, targetType: "",
		},
	}

	for label, testCase := range testCases {
		t.Run(label, func(t *testing.T) {
			if _, err := domainaudit.NewAuditLog(
				testCase.publicID, testCase.userID, testCase.action,
				testCase.targetType, nil, nil, testNow); err == nil {
				t.Fatal("NewAuditLog returned nil error, want error")
			}
		})
	}
}

func TestDiff_作成時は全項目をFrom_nilで記録する(t *testing.T) {
	after := map[string]any{"name": "折りたたみ傘", "quantity": int32(1)}

	changes := domainaudit.Diff(nil, after)

	if len(changes) != 2 {
		t.Fatalf("len(changes) = %d, want 2", len(changes))
	}
	if changes["name"].From != nil {
		t.Errorf("changes[name].From = %v, want nil", changes["name"].From)
	}
	if changes["name"].To != "折りたたみ傘" {
		t.Errorf("changes[name].To = %v, want 折りたたみ傘", changes["name"].To)
	}
}

func TestDiff_変更のあった項目だけを記録する(t *testing.T) {
	before := map[string]any{
		"name":     "折りたたみ傘",
		"quantity": int32(1),
		"notes":    (*string)(nil),
	}
	after := map[string]any{
		"name":     "長傘",
		"quantity": int32(1),
		"notes":    (*string)(nil),
	}

	changes := domainaudit.Diff(before, after)

	if len(changes) != 1 {
		t.Fatalf("len(changes) = %d, want 1 (%+v)", len(changes), changes)
	}
	if changes["name"].From != "折りたたみ傘" || changes["name"].To != "長傘" {
		t.Errorf("changes[name] = %+v, want 折りたたみ傘 -> 長傘", changes["name"])
	}
}

func TestDiff_pointerの中身で比較する(t *testing.T) {
	before := map[string]any{
		"notes":           pointerTo("古いメモ"),
		"desiredQuantity": pointerTo(int32(2)),
	}
	after := map[string]any{
		"notes":           pointerTo("古いメモ"),
		"desiredQuantity": pointerTo(int32(3)),
	}

	changes := domainaudit.Diff(before, after)

	if _, changed := changes["notes"]; changed {
		t.Error("changes contains notes, want unchanged")
	}
	if _, changed := changes["desiredQuantity"]; !changed {
		t.Error("changes does not contain desiredQuantity, want changed")
	}
}

func TestDiff_nilとの差分を検出する(t *testing.T) {
	before := map[string]any{"notes": (*string)(nil)}
	after := map[string]any{"notes": pointerTo("新しいメモ")}

	changes := domainaudit.Diff(before, after)

	if _, changed := changes["notes"]; !changed {
		t.Error("changes does not contain notes, want changed")
	}
}

func TestDiff_文字列sliceを比較する(t *testing.T) {
	before := map[string]any{"tagPublicIds": []string{"a", "b"}}

	unchanged := domainaudit.Diff(before, map[string]any{"tagPublicIds": []string{"a", "b"}})
	if _, changed := unchanged["tagPublicIds"]; changed {
		t.Error("changes contains tagPublicIds, want unchanged")
	}

	changed := domainaudit.Diff(before, map[string]any{"tagPublicIds": []string{"a"}})
	if _, ok := changed["tagPublicIds"]; !ok {
		t.Error("changes does not contain tagPublicIds, want changed")
	}
}
