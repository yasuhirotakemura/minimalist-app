// Package audit は操作履歴 (監査ログ) を提供する (設計書 22章)。
//
// 設計書 5.1 のpackage構成表には現れないが、監査ログは独自のtableと
// 永続化責務を持つため独立したpackageとした。
//
// 方針:
//   - 追記のみのEntityとし、更新・削除を行わない。
//   - changesは差分のみを保持する。
//   - 機微情報 (password、session token、CSRF secret等) を保持しない。
package audit

import (
	"time"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// ActionCode は記録対象の操作 (設計書 22章)。
//
// Phase 1で記録する操作のみを定義する。
// 後続phaseで対象が増えるため、DB側は値集合ではなく形式をCHECKで保証する。
type ActionCode string

// ActionCodeの値。
const (
	ActionDefaultCategoriesCreated ActionCode = "default_categories_created"
	ActionItemCreated              ActionCode = "item_created"
	ActionItemUpdated              ActionCode = "item_updated"
	ActionItemArchived             ActionCode = "item_archived"
	ActionItemRestored             ActionCode = "item_restored"
	ActionItemUsageRecorded        ActionCode = "item_usage_recorded"
	ActionTagCreated               ActionCode = "tag_created"
	ActionTagUpdated               ActionCode = "tag_updated"
	ActionTagDeleted               ActionCode = "tag_deleted"
)

// String はcodeを返す。
func (c ActionCode) String() string { return string(c) }

// TargetTypeCode は操作対象の種別。
type TargetTypeCode string

// TargetTypeCodeの値。
const (
	TargetTypeItem     TargetTypeCode = "item"
	TargetTypeTag      TargetTypeCode = "tag"
	TargetTypeCategory TargetTypeCode = "category"
)

// String はcodeを返す。
func (c TargetTypeCode) String() string { return string(c) }

// FieldChange は1項目の変更前後を表す。
type FieldChange struct {
	From any `json:"from"`
	To   any `json:"to"`
}

// Changes は項目名から変更内容への対応。
//
// JSONBへ保存するため、値はJSONへ変換可能な型のみを含める。
type Changes map[string]FieldChange

// AuditLog は操作履歴の1件。
type AuditLog struct {
	publicID       uuid.UUID
	userID         auth.UserID
	action         ActionCode
	targetType     TargetTypeCode
	targetPublicID *uuid.UUID
	changes        Changes
	createdAt      time.Time
}

// NewAuditLog は操作履歴を生成する。
func NewAuditLog(
	publicID uuid.UUID,
	userID auth.UserID,
	action ActionCode,
	targetType TargetTypeCode,
	targetPublicID *uuid.UUID,
	changes Changes,
	now time.Time,
) (AuditLog, error) {
	if publicID == uuid.Nil {
		return AuditLog{}, shared.NewInternalError(
			"INVALID_PUBLIC_ID", "内部エラーが発生しました。")
	}
	if userID.IsZero() {
		return AuditLog{}, shared.NewInternalError(
			"INVALID_USER_ID", "内部エラーが発生しました。")
	}
	if action == "" || targetType == "" {
		return AuditLog{}, shared.NewInternalError(
			"INVALID_AUDIT_LOG", "内部エラーが発生しました。")
	}

	if changes == nil {
		changes = Changes{}
	}

	return AuditLog{
		publicID:       publicID,
		userID:         userID,
		action:         action,
		targetType:     targetType,
		targetPublicID: targetPublicID,
		changes:        changes,
		createdAt:      now.UTC(),
	}, nil
}

// PublicID は外部公開IDを返す。
func (l AuditLog) PublicID() uuid.UUID { return l.publicID }

// UserID は操作したユーザーの内部IDを返す。
func (l AuditLog) UserID() auth.UserID { return l.userID }

// Action は操作codeを返す。
func (l AuditLog) Action() ActionCode { return l.action }

// TargetType は対象種別を返す。
func (l AuditLog) TargetType() TargetTypeCode { return l.targetType }

// TargetPublicID は対象の外部公開IDを返す。対象が単一でない場合はnil。
func (l AuditLog) TargetPublicID() *uuid.UUID { return l.targetPublicID }

// Changes は差分を返す。
func (l AuditLog) Changes() Changes { return l.changes }

// CreatedAt は記録日時を返す。
func (l AuditLog) CreatedAt() time.Time { return l.createdAt }

// Diff は変更前後のsnapshotから差分のみを抽出する (設計書 22章)。
//
// 変更が無い項目は含めない。beforeがnilの場合は作成として扱い、
// afterの全項目を From=nil として記録する。
func Diff(before, after map[string]any) Changes {
	changes := Changes{}

	if before == nil {
		for field, value := range after {
			changes[field] = FieldChange{From: nil, To: value}
		}
		return changes
	}

	for field, afterValue := range after {
		beforeValue, existed := before[field]
		if existed && equalSnapshotValues(beforeValue, afterValue) {
			continue
		}
		changes[field] = FieldChange{From: beforeValue, To: afterValue}
	}
	return changes
}

// equalSnapshotValues はsnapshotの値が等しいかを判定する。
//
// snapshotはpointerとsliceを含むため、reflect.DeepEqualではなく
// 値の中身で比較する。
func equalSnapshotValues(left, right any) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}

	leftValues, leftIsSlice := left.([]string)
	rightValues, rightIsSlice := right.([]string)
	if leftIsSlice || rightIsSlice {
		if !leftIsSlice || !rightIsSlice || len(leftValues) != len(rightValues) {
			return false
		}
		for index := range leftValues {
			if leftValues[index] != rightValues[index] {
				return false
			}
		}
		return true
	}

	return dereference(left) == dereference(right)
}

// dereference はsnapshotに含まれるpointerを値へ展開する。
func dereference(value any) any {
	switch typed := value.(type) {
	case *string:
		if typed == nil {
			return nil
		}
		return *typed
	case *int32:
		if typed == nil {
			return nil
		}
		return *typed
	case *int64:
		if typed == nil {
			return nil
		}
		return *typed
	default:
		return value
	}
}
