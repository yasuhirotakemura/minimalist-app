package storage

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"

	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

// recordVersionConflict は楽観ロック競合を監査ログへ記録し、元のerrorを返す
// (設計書 11.7 / 22章)。
//
// 競合が起きたtransactionはrollbackするため、業務操作と同じtransactionでは
// 記録が残らない。競合の検知だけを別transactionで記録する。
//
// 記録自体の失敗で利用者へ返すerrorをすり替えない。競合の事実を伝えることが
// 優先であり、記録漏れは業務結果を変えない。
func recordVersionConflict(
	ctx context.Context,
	dependencies Dependencies,
	transactionManager transaction.Manager,
	userID domainauth.UserID,
	targetPublicID uuid.UUID,
	operation string,
	cause error,
) error {
	if !isVersionConflict(cause) {
		return cause
	}

	recordErr := transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		return dependencies.AuditRecorder.Record(
			ctx,
			userID,
			domainaudit.ActionVersionConflictDetected,
			domainaudit.TargetTypeStorageUnit,
			&targetPublicID,
			domainaudit.Changes{
				"operation": domainaudit.FieldChange{From: nil, To: operation},
			},
		)
	})
	_ = recordErr

	return cause
}

// isVersionConflict は収納単位・収納割当の楽観ロック競合かどうかを返す。
func isVersionConflict(err error) bool {
	return errors.Is(err, domainstorage.ErrStorageUnitVersionConflict) ||
		errors.Is(err, domainstorage.ErrStorageAllocationVersionConflict)
}

// capacityChanges は容量超過状態の変化を監査ログの差分として返す (設計書 16.3 / 22章)。
//
// 超過状態へ入った・出たことを記録し、いつ過積載になったかを追跡できるようにする。
func capacityChanges(before, after domainstorage.Capacity) domainaudit.Changes {
	changes := domainaudit.Changes{}
	if before.IsWeightExceeded != after.IsWeightExceeded {
		changes["isWeightExceeded"] = domainaudit.FieldChange{
			From: before.IsWeightExceeded, To: after.IsWeightExceeded,
		}
	}
	if before.IsVolumeExceeded != after.IsVolumeExceeded {
		changes["isVolumeExceeded"] = domainaudit.FieldChange{
			From: before.IsVolumeExceeded, To: after.IsVolumeExceeded,
		}
	}
	return changes
}

// mergeChanges は差分を1つのChangesへまとめる。
func mergeChanges(base domainaudit.Changes, extra domainaudit.Changes) domainaudit.Changes {
	if base == nil {
		base = domainaudit.Changes{}
	}
	for field, change := range extra {
		base[field] = change
	}
	return base
}

// itoa は数量を文字列へ変換する。監査ログの要約表現で使用する。
func itoa(value int32) string { return strconv.FormatInt(int64(value), 10) }
