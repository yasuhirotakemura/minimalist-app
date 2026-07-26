-- 監査ログ (設計書 22章)。
-- 追記のみのtableであり、更新・削除queryを持たない。
-- changesは差分のみを保持し、機微情報を含めない。

-- name: InsertAuditLog :exec
INSERT INTO audit.audit_logs (
    public_id,
    user_id,
    action_code,
    target_type_code,
    target_public_id,
    changes,
    created_at
) VALUES (
    @public_id,
    @user_id,
    @action_code,
    @target_type_code,
    @target_public_id,
    @changes,
    @created_at
);
