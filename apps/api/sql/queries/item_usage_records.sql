-- 全てのユーザーdata queryは user internal ID を条件に含める (設計書 11.6 / 18.3)。

-- name: InsertItemUsageRecord :one
INSERT INTO ownership.item_usage_records (
    public_id,
    user_id,
    item_id,
    used_at,
    quantity,
    note,
    created_at
) VALUES (
    @public_id,
    @user_id,
    @item_id,
    @used_at,
    @quantity,
    @note,
    @created_at
)
RETURNING *;

-- 履歴は使用日時の降順で返す。同時刻はid降順で安定させる。
-- name: ListItemUsageRecordsByItemID :many
SELECT *
FROM ownership.item_usage_records
WHERE user_id = @user_id
  AND item_id = @item_id
ORDER BY used_at DESC, id DESC
LIMIT @row_limit
OFFSET @row_offset;

-- name: CountItemUsageRecordsByItemID :one
SELECT COUNT(*)
FROM ownership.item_usage_records
WHERE user_id = @user_id
  AND item_id = @item_id;
