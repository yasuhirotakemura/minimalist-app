package audit

import "context"

// AuditLogRepository は操作履歴の永続化を担う (設計書 6.5 / 11.6)。
//
// 追記のみのため取得系methodを持たない。
// 操作履歴の参照 (F-020) は設計書 12.4 にendpointが定義されておらず、
// 本phaseのスコープ外とする。
type AuditLogRepository interface {
	// Create は操作履歴を1件記録する。
	//
	// 呼び出し元の業務操作と同一transaction内で実行する (設計書 20.1)。
	Create(ctx context.Context, log AuditLog) error
}
