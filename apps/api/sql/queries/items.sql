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
    desired_quantity,
    unit_name,
    necessity_level_code,
    usage_frequency_code,
    substitutability_code,
    mobility_class_code,
    ownership_reason,
    disposal_condition,
    last_used_at,
    purchased_on,
    purchase_amount,
    replacement_amount,
    resale_amount,
    weight_gram,
    volume_milliliter,
    is_fragile,
    is_valuable,
    is_sentimental,
    requires_maintenance,
    expires_on,
    source_url,
    notes,
    is_confirmed,
    confirmed_at,
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
    @desired_quantity,
    @unit_name,
    @necessity_level_code,
    @usage_frequency_code,
    @substitutability_code,
    @mobility_class_code,
    @ownership_reason,
    @disposal_condition,
    @last_used_at,
    @purchased_on,
    @purchase_amount,
    @replacement_amount,
    @resale_amount,
    @weight_gram,
    @volume_milliliter,
    @is_fragile,
    @is_valuable,
    @is_sentimental,
    @requires_maintenance,
    @expires_on,
    @source_url,
    @notes,
    @is_confirmed,
    @confirmed_at,
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
SET category_id           = @category_id,
    name                  = @name,
    item_kind_code        = @item_kind_code,
    quantity              = @quantity,
    desired_quantity      = @desired_quantity,
    unit_name             = @unit_name,
    necessity_level_code  = @necessity_level_code,
    usage_frequency_code  = @usage_frequency_code,
    substitutability_code = @substitutability_code,
    mobility_class_code   = @mobility_class_code,
    ownership_reason      = @ownership_reason,
    disposal_condition    = @disposal_condition,
    last_used_at          = @last_used_at,
    purchased_on          = @purchased_on,
    purchase_amount       = @purchase_amount,
    replacement_amount    = @replacement_amount,
    resale_amount         = @resale_amount,
    weight_gram           = @weight_gram,
    volume_milliliter     = @volume_milliliter,
    is_fragile            = @is_fragile,
    is_valuable           = @is_valuable,
    is_sentimental        = @is_sentimental,
    requires_maintenance  = @requires_maintenance,
    expires_on            = @expires_on,
    source_url            = @source_url,
    notes                 = @notes,
    updated_at            = @updated_at,
    version               = version + 1
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

-- 使用記録の登録に伴い最終使用日時を更新する。
-- 既存値より古い使用日時では最終使用日時を後退させない。
-- 使用記録は追記操作のため expectedVersion を要求しないが、
-- 見直しスコアの再計算契機となるため version は増加させる。
-- name: TouchItemLastUsedAt :one
UPDATE ownership.items
SET last_used_at = GREATEST(COALESCE(last_used_at, @used_at), @used_at),
    updated_at   = @updated_at,
    version      = version + 1
WHERE public_id = @public_id
  AND user_id = @user_id
  AND deleted_at IS NULL
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
  AND (sqlc.narg('mobility_class_code')::text IS NULL
       OR i.mobility_class_code = sqlc.narg('mobility_class_code')::text)
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
    CASE WHEN @sort_key::text = 'last_used_at' AND NOT @descending::boolean
         THEN i.last_used_at END ASC NULLS LAST,
    CASE WHEN @sort_key::text = 'last_used_at' AND @descending::boolean
         THEN i.last_used_at END DESC NULLS LAST,
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
  AND (sqlc.narg('mobility_class_code')::text IS NULL
       OR i.mobility_class_code = sqlc.narg('mobility_class_code')::text)
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
