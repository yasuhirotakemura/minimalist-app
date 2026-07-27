-- 全てのユーザーdata queryは user internal ID を条件に含める (設計書 11.6 / 18.3)。
-- 楽観ロックは version を WHERE 条件へ含め、更新件数0を競合として扱う (設計書 11.7)。

-- name: InsertItem :one
INSERT INTO ownership.items (
    public_id,
    user_id,
    category_id,
    name,
    item_kind_code,
    quantity,
    unit_name,
    necessity_level_code,
    usage_frequency_code,
    purchased_on,
    source_url,
    notes,
    created_at,
    updated_at,
    version
) VALUES (
    @public_id,
    @user_id,
    @category_id,
    @name,
    @item_kind_code,
    @quantity,
    @unit_name,
    @necessity_level_code,
    @usage_frequency_code,
    @purchased_on,
    @source_url,
    @notes,
    @created_at,
    @updated_at,
    1
)
RETURNING *;

-- archive済み (deleted_at IS NOT NULL) も返す。
-- 詳細画面からの復元と、archive状態の表示に使用する。
-- name: FindItemByPublicID :one
SELECT
    sqlc.embed(i),
    c.public_id AS category_public_id,
    c.name      AS category_name
FROM ownership.items i
JOIN ownership.categories c
  ON c.id = i.category_id
 AND c.user_id = i.user_id
WHERE i.public_id = @public_id
  AND i.user_id = @user_id;

-- 全項目を置き換える。versionが一致しない場合は0件となる。
-- name: UpdateItem :one
UPDATE ownership.items
SET category_id          = @category_id,
    name                 = @name,
    item_kind_code       = @item_kind_code,
    quantity             = @quantity,
    unit_name            = @unit_name,
    necessity_level_code = @necessity_level_code,
    usage_frequency_code = @usage_frequency_code,
    purchased_on         = @purchased_on,
    source_url           = @source_url,
    notes                = @notes,
    updated_at           = @updated_at,
    version              = version + 1
WHERE public_id = @public_id
  AND user_id = @user_id
  AND deleted_at IS NULL
  AND version = @expected_version
RETURNING *;

-- archiveはsoft deleteとして表現する (設計書 1.4 / 12.4)。
-- name: ArchiveItem :one
UPDATE ownership.items
SET deleted_at = @archived_at,
    updated_at = @updated_at,
    version    = version + 1
WHERE public_id = @public_id
  AND user_id = @user_id
  AND deleted_at IS NULL
  AND version = @expected_version
RETURNING *;

-- name: RestoreItem :one
UPDATE ownership.items
SET deleted_at = NULL,
    updated_at = @updated_at,
    version    = version + 1
WHERE public_id = @public_id
  AND user_id = @user_id
  AND deleted_at IS NOT NULL
  AND version = @expected_version
RETURNING *;

-- 更新件数0の理由 (不存在か競合か) を判定するために使用する。
-- name: ExistsItemByPublicID :one
SELECT EXISTS (
    SELECT 1
    FROM ownership.items
    WHERE public_id = @public_id
      AND user_id = @user_id
) AS exists;

-- 所持品一覧 (設計書 9.4)。
--
-- 並び替えはsqlcで静的に生成するため、ORDER BYへCASEを並べて表現する。
-- sort keyに一致しないCASEは全行NULLとなり順序へ影響しない。
-- 同値時の順序を安定させるため、最後に id を加える。
-- name: ListItems :many
SELECT
    sqlc.embed(i),
    c.public_id AS category_public_id,
    c.name      AS category_name
FROM ownership.items i
JOIN ownership.categories c
  ON c.id = i.category_id
 AND c.user_id = i.user_id
WHERE i.user_id = @user_id
  AND (@include_deleted::boolean OR i.deleted_at IS NULL)
  AND (
    sqlc.narg('keyword_pattern')::text IS NULL
    OR i.name ILIKE sqlc.narg('keyword_pattern')::text
    OR i.notes ILIKE sqlc.narg('keyword_pattern')::text
  )
  AND (sqlc.narg('category_public_id')::uuid IS NULL
       OR c.public_id = sqlc.narg('category_public_id')::uuid)
  AND (sqlc.narg('necessity_level_code')::text IS NULL
       OR i.necessity_level_code = sqlc.narg('necessity_level_code')::text)
  AND (sqlc.narg('usage_frequency_code')::text IS NULL
       OR i.usage_frequency_code = sqlc.narg('usage_frequency_code')::text)
  AND (sqlc.narg('tag_public_id')::uuid IS NULL
       OR EXISTS (
            SELECT 1
            FROM ownership.item_tags it
            JOIN ownership.tags t
              ON t.id = it.tag_id
             AND t.user_id = it.user_id
             AND t.deleted_at IS NULL
            WHERE it.item_id = i.id
              AND it.user_id = i.user_id
              AND t.public_id = sqlc.narg('tag_public_id')::uuid))
ORDER BY
    CASE WHEN @sort_key::text = 'name' AND NOT @descending::boolean
         THEN i.name END ASC,
    CASE WHEN @sort_key::text = 'name' AND @descending::boolean
         THEN i.name END DESC,
    CASE WHEN @sort_key::text = 'quantity' AND NOT @descending::boolean
         THEN i.quantity END ASC,
    CASE WHEN @sort_key::text = 'quantity' AND @descending::boolean
         THEN i.quantity END DESC,
    CASE WHEN @sort_key::text = 'updated_at' AND NOT @descending::boolean
         THEN i.updated_at END ASC,
    CASE WHEN @sort_key::text = 'updated_at' AND @descending::boolean
         THEN i.updated_at END DESC,
    i.id DESC
LIMIT @row_limit
OFFSET @row_offset;

-- ListItemsと同一の絞り込み条件で総件数を返す。
-- name: CountItems :one
SELECT COUNT(*)
FROM ownership.items i
JOIN ownership.categories c
  ON c.id = i.category_id
 AND c.user_id = i.user_id
WHERE i.user_id = @user_id
  AND (@include_deleted::boolean OR i.deleted_at IS NULL)
  AND (
    sqlc.narg('keyword_pattern')::text IS NULL
    OR i.name ILIKE sqlc.narg('keyword_pattern')::text
    OR i.notes ILIKE sqlc.narg('keyword_pattern')::text
  )
  AND (sqlc.narg('category_public_id')::uuid IS NULL
       OR c.public_id = sqlc.narg('category_public_id')::uuid)
  AND (sqlc.narg('necessity_level_code')::text IS NULL
       OR i.necessity_level_code = sqlc.narg('necessity_level_code')::text)
  AND (sqlc.narg('usage_frequency_code')::text IS NULL
       OR i.usage_frequency_code = sqlc.narg('usage_frequency_code')::text)
  AND (sqlc.narg('tag_public_id')::uuid IS NULL
       OR EXISTS (
            SELECT 1
            FROM ownership.item_tags it
            JOIN ownership.tags t
              ON t.id = it.tag_id
             AND t.user_id = it.user_id
             AND t.deleted_at IS NULL
            WHERE it.item_id = i.id
              AND it.user_id = i.user_id
              AND t.public_id = sqlc.narg('tag_public_id')::uuid));

-- ダッシュボードの合計 (設計書 9.3)。
--
-- item_type_count はアイテム種別 (行) の数、total_quantity は所有数量の合計。
-- archive済みは集計へ含めない。
-- name: AggregateItemTotals :one
SELECT
    COUNT(*)::bigint                        AS item_type_count,
    COALESCE(SUM(quantity), 0)::bigint      AS total_quantity
FROM ownership.items
WHERE user_id = @user_id
  AND deleted_at IS NULL;

-- ダッシュボードのカテゴリー別内訳 (設計書 9.3)。
--
-- 所持品が1件も無いカテゴリーは行として返さない。
-- 並びはカテゴリーの表示順 (sort_order) とする。
-- name: AggregateItemCountsByCategory :many
SELECT
    c.public_id                        AS category_public_id,
    c.name                             AS category_name,
    COUNT(i.id)::bigint                AS item_type_count,
    COALESCE(SUM(i.quantity), 0)::bigint AS total_quantity
FROM ownership.categories c
JOIN ownership.items i
  ON i.category_id = c.id
 AND i.user_id = c.user_id
 AND i.deleted_at IS NULL
WHERE c.user_id = @user_id
  AND c.deleted_at IS NULL
GROUP BY c.id, c.public_id, c.name, c.sort_order
ORDER BY c.sort_order, c.id;

-- ダッシュボードの必要度別内訳 (設計書 9.3)。
-- 表示順はDomainのcode体系が決めるため、SQLではcode順に返す。
-- name: AggregateItemCountsByNecessityLevel :many
SELECT
    necessity_level_code,
    COUNT(*)::bigint                   AS item_type_count,
    COALESCE(SUM(quantity), 0)::bigint AS total_quantity
FROM ownership.items
WHERE user_id = @user_id
  AND deleted_at IS NULL
GROUP BY necessity_level_code
ORDER BY necessity_level_code;

-- ダッシュボードの使用頻度別内訳 (設計書 9.3)。
-- name: AggregateItemCountsByUsageFrequency :many
SELECT
    usage_frequency_code,
    COUNT(*)::bigint                   AS item_type_count,
    COALESCE(SUM(quantity), 0)::bigint AS total_quantity
FROM ownership.items
WHERE user_id = @user_id
  AND deleted_at IS NULL
GROUP BY usage_frequency_code
ORDER BY usage_frequency_code;

-- 一覧のN+1を避けるため、pageに含まれるアイテムのタグをまとめて取得する。
-- name: ListItemTagsByItemIDs :many
SELECT
    it.item_id,
    t.public_id,
    t.name
FROM ownership.item_tags it
JOIN ownership.tags t
  ON t.id = it.tag_id
 AND t.user_id = it.user_id
 AND t.deleted_at IS NULL
WHERE it.user_id = @user_id
  AND it.item_id = ANY(@item_ids::bigint[])
ORDER BY t.name, t.id;

-- name: DeleteItemTagsByItemID :exec
DELETE FROM ownership.item_tags
WHERE user_id = @user_id
  AND item_id = @item_id;

-- name: InsertItemTags :exec
INSERT INTO ownership.item_tags (user_id, item_id, tag_id)
SELECT @user_id, @item_id, tag_id
FROM unnest(@tag_ids::bigint[]) AS tag_id
ON CONFLICT DO NOTHING;
