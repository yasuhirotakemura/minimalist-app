-- 全てのユーザーdata queryは user internal ID を条件に含める (設計書 11.6 / 18.3)。

-- name: InsertTag :one
INSERT INTO ownership.tags (
    public_id,
    user_id,
    name,
    created_at,
    updated_at,
    version
) VALUES (
    @public_id,
    @user_id,
    @name,
    @created_at,
    @updated_at,
    1
)
RETURNING *;

-- 一覧では付与済みアイテム件数を併せて返す。
-- archive済みアイテムは件数へ含めない。
-- name: ListActiveTagsWithItemCountByUserID :many
SELECT
    sqlc.embed(t),
    COUNT(i.id) AS item_count
FROM ownership.tags t
LEFT JOIN ownership.item_tags it
       ON it.tag_id = t.id
      AND it.user_id = t.user_id
LEFT JOIN ownership.items i
       ON i.id = it.item_id
      AND i.user_id = t.user_id
      AND i.deleted_at IS NULL
WHERE t.user_id = @user_id
  AND t.deleted_at IS NULL
GROUP BY t.id
ORDER BY t.name, t.id;

-- name: FindActiveTagByPublicID :one
SELECT *
FROM ownership.tags
WHERE public_id = @public_id
  AND user_id = @user_id
  AND deleted_at IS NULL;

-- アイテムへ付与するタグをまとめて解決する。
-- 指定した件数と結果件数が一致しない場合、呼び出し側は不存在として扱う。
-- name: ListActiveTagsByPublicIDs :many
SELECT *
FROM ownership.tags
WHERE user_id = @user_id
  AND public_id = ANY(@public_ids::uuid[])
  AND deleted_at IS NULL
ORDER BY name, id;

-- name: CountActiveItemsByTagID :one
SELECT COUNT(i.id)
FROM ownership.item_tags it
JOIN ownership.items i
  ON i.id = it.item_id
 AND i.user_id = it.user_id
 AND i.deleted_at IS NULL
WHERE it.tag_id = @tag_id
  AND it.user_id = @user_id;

-- 楽観ロック (設計書 11.7)。更新件数が0の場合は競合または不存在として扱う。
-- name: UpdateTag :one
UPDATE ownership.tags
SET name       = @name,
    updated_at = @updated_at,
    version    = version + 1
WHERE public_id = @public_id
  AND user_id = @user_id
  AND deleted_at IS NULL
  AND version = @expected_version
RETURNING *;

-- name: SoftDeleteTag :one
UPDATE ownership.tags
SET deleted_at = @deleted_at,
    updated_at = @updated_at,
    version    = version + 1
WHERE public_id = @public_id
  AND user_id = @user_id
  AND deleted_at IS NULL
  AND version = @expected_version
RETURNING *;

-- 更新件数0の理由 (不存在か競合か) を判定するために使用する。
-- name: ExistsActiveTagByPublicID :one
SELECT EXISTS (
    SELECT 1
    FROM ownership.tags
    WHERE public_id = @public_id
      AND user_id = @user_id
      AND deleted_at IS NULL
) AS exists;
