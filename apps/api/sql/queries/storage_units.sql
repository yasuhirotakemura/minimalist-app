-- 全てのユーザーdata queryは user internal ID を条件に含める (設計書 11.6 / 18.3)。
-- 楽観ロックは version を WHERE 条件へ含め、更新件数0を競合として扱う (設計書 11.7)。
--
-- 祖先の取得について:
--   階層上限が3であるため (設計書 7.3)、親・祖父母の2段をLEFT JOINすれば
--   rootまでの並びが得られる。recursive CTEを避け、planを単純に保つ。
--   階層上限はDomainが保証する。

-- name: InsertStorageUnit :one
INSERT INTO ownership.storage_units (
    public_id,
    user_id,
    parent_id,
    name,
    storage_type_code,
    mobility_class_code,
    tare_weight_gram,
    maximum_weight_gram,
    maximum_volume_milliliter,
    description,
    sort_order,
    created_at,
    updated_at,
    version
) VALUES (
    @public_id,
    @user_id,
    @parent_id,
    @name,
    @storage_type_code,
    @mobility_class_code,
    @tare_weight_gram,
    @maximum_weight_gram,
    @maximum_volume_milliliter,
    @description,
    @sort_order,
    @created_at,
    @updated_at,
    1
)
RETURNING *;

-- archive済み (deleted_at IS NOT NULL) も返す。
-- 詳細画面からの復元と、archive状態の表示に使用する。
-- name: FindStorageUnitByPublicID :one
SELECT
    sqlc.embed(s),
    p.public_id  AS parent_public_id,
    p.name       AS parent_name,
    gp.id        AS grandparent_id,
    gp.public_id AS grandparent_public_id,
    gp.name      AS grandparent_name,
    (
        SELECT COUNT(*)
        FROM ownership.storage_units c
        WHERE c.parent_id = s.id
          AND c.user_id = s.user_id
          AND c.deleted_at IS NULL
    )::bigint AS child_count
FROM ownership.storage_units s
LEFT JOIN ownership.storage_units p
  ON p.id = s.parent_id
 AND p.user_id = s.user_id
LEFT JOIN ownership.storage_units gp
  ON gp.id = p.parent_id
 AND gp.user_id = s.user_id
WHERE s.public_id = @public_id
  AND s.user_id = @user_id;

-- 全項目を置き換える。versionが一致しない場合は0件となる。
-- name: UpdateStorageUnit :one
UPDATE ownership.storage_units
SET parent_id                 = @parent_id,
    name                      = @name,
    storage_type_code         = @storage_type_code,
    mobility_class_code       = @mobility_class_code,
    tare_weight_gram          = @tare_weight_gram,
    maximum_weight_gram       = @maximum_weight_gram,
    maximum_volume_milliliter = @maximum_volume_milliliter,
    description               = @description,
    sort_order                = @sort_order,
    updated_at                = @updated_at,
    version                   = version + 1
WHERE public_id = @public_id
  AND user_id = @user_id
  AND deleted_at IS NULL
  AND version = @expected_version
RETURNING *;

-- archiveはsoft deleteとして表現する (設計書 1.4)。
-- name: ArchiveStorageUnit :one
UPDATE ownership.storage_units
SET deleted_at = @archived_at,
    updated_at = @updated_at,
    version    = version + 1
WHERE public_id = @public_id
  AND user_id = @user_id
  AND deleted_at IS NULL
  AND version = @expected_version
RETURNING id;

-- name: RestoreStorageUnit :one
UPDATE ownership.storage_units
SET deleted_at = NULL,
    updated_at = @updated_at,
    version    = version + 1
WHERE public_id = @public_id
  AND user_id = @user_id
  AND deleted_at IS NOT NULL
  AND version = @expected_version
RETURNING id;

-- 収納割当の追加・変更・削除・一括置換に伴い収納単位のversionを増加させる。
-- 割当集合全体の競合を収納単位のversionで検知する (設計書 11.7)。
-- name: TouchStorageUnitVersion :one
UPDATE ownership.storage_units
SET updated_at = @updated_at,
    version    = version + 1
WHERE public_id = @public_id
  AND user_id = @user_id
  AND deleted_at IS NULL
  AND version = @expected_version
RETURNING id;

-- 更新件数0の理由 (不存在か競合か) を判定するために使用する。
-- name: ExistsStorageUnitByPublicID :one
SELECT EXISTS (
    SELECT 1
    FROM ownership.storage_units
    WHERE public_id = @public_id
      AND user_id = @user_id
) AS exists;

-- 収納単位一覧 (設計書 9.4 相当)。
--
-- 並び替えはsqlcで静的に生成するため、ORDER BYへCASEを並べて表現する。
-- 同値時の順序を安定させるため、最後に id を加える。
-- name: ListStorageUnits :many
SELECT
    sqlc.embed(s),
    p.public_id  AS parent_public_id,
    p.name       AS parent_name,
    gp.id        AS grandparent_id,
    gp.public_id AS grandparent_public_id,
    gp.name      AS grandparent_name,
    (
        SELECT COUNT(*)
        FROM ownership.storage_units c
        WHERE c.parent_id = s.id
          AND c.user_id = s.user_id
          AND c.deleted_at IS NULL
    )::bigint AS child_count
FROM ownership.storage_units s
LEFT JOIN ownership.storage_units p
  ON p.id = s.parent_id
 AND p.user_id = s.user_id
LEFT JOIN ownership.storage_units gp
  ON gp.id = p.parent_id
 AND gp.user_id = s.user_id
WHERE s.user_id = @user_id
  AND (@include_archived::boolean OR s.deleted_at IS NULL)
  AND (
    sqlc.narg('keyword_pattern')::text IS NULL
    OR s.name ILIKE sqlc.narg('keyword_pattern')::text
    OR s.description ILIKE sqlc.narg('keyword_pattern')::text
  )
  AND (sqlc.narg('storage_type_code')::text IS NULL
       OR s.storage_type_code = sqlc.narg('storage_type_code')::text)
  AND (sqlc.narg('mobility_class_code')::text IS NULL
       OR s.mobility_class_code = sqlc.narg('mobility_class_code')::text)
  AND (sqlc.narg('parent_public_id')::uuid IS NULL
       OR p.public_id = sqlc.narg('parent_public_id')::uuid)
  AND (NOT @root_only::boolean OR s.parent_id IS NULL)
ORDER BY
    CASE WHEN @sort_key::text = 'name' AND NOT @descending::boolean
         THEN s.name END ASC,
    CASE WHEN @sort_key::text = 'name' AND @descending::boolean
         THEN s.name END DESC,
    CASE WHEN @sort_key::text = 'sort_order' AND NOT @descending::boolean
         THEN s.sort_order END ASC,
    CASE WHEN @sort_key::text = 'sort_order' AND @descending::boolean
         THEN s.sort_order END DESC,
    CASE WHEN @sort_key::text = 'updated_at' AND NOT @descending::boolean
         THEN s.updated_at END ASC,
    CASE WHEN @sort_key::text = 'updated_at' AND @descending::boolean
         THEN s.updated_at END DESC,
    s.id ASC
LIMIT @row_limit
OFFSET @row_offset;

-- ListStorageUnitsと同一の絞り込み条件で総件数を返す。
-- name: CountStorageUnits :one
SELECT COUNT(*)
FROM ownership.storage_units s
LEFT JOIN ownership.storage_units p
  ON p.id = s.parent_id
 AND p.user_id = s.user_id
WHERE s.user_id = @user_id
  AND (@include_archived::boolean OR s.deleted_at IS NULL)
  AND (
    sqlc.narg('keyword_pattern')::text IS NULL
    OR s.name ILIKE sqlc.narg('keyword_pattern')::text
    OR s.description ILIKE sqlc.narg('keyword_pattern')::text
  )
  AND (sqlc.narg('storage_type_code')::text IS NULL
       OR s.storage_type_code = sqlc.narg('storage_type_code')::text)
  AND (sqlc.narg('mobility_class_code')::text IS NULL
       OR s.mobility_class_code = sqlc.narg('mobility_class_code')::text)
  AND (sqlc.narg('parent_public_id')::uuid IS NULL
       OR p.public_id = sqlc.narg('parent_public_id')::uuid)
  AND (NOT @root_only::boolean OR s.parent_id IS NULL);

-- 階層をまたぐ容量集計と循環参照の検証は木全体を必要とするため、
-- pageに含まれない収納単位も取得する (設計書 16.2)。
-- name: ListAllStorageUnits :many
SELECT
    sqlc.embed(s),
    p.public_id  AS parent_public_id,
    p.name       AS parent_name,
    gp.id        AS grandparent_id,
    gp.public_id AS grandparent_public_id,
    gp.name      AS grandparent_name,
    (
        SELECT COUNT(*)
        FROM ownership.storage_units c
        WHERE c.parent_id = s.id
          AND c.user_id = s.user_id
          AND c.deleted_at IS NULL
    )::bigint AS child_count
FROM ownership.storage_units s
LEFT JOIN ownership.storage_units p
  ON p.id = s.parent_id
 AND p.user_id = s.user_id
LEFT JOIN ownership.storage_units gp
  ON gp.id = p.parent_id
 AND gp.user_id = s.user_id
WHERE s.user_id = @user_id
  AND (@include_archived::boolean OR s.deleted_at IS NULL)
ORDER BY s.sort_order ASC, s.id ASC;

-- 詳細画面で直接の子収納単位を表示する。archive済みは含めない。
-- name: ListChildStorageUnits :many
SELECT
    sqlc.embed(s),
    p.public_id  AS parent_public_id,
    p.name       AS parent_name,
    gp.id        AS grandparent_id,
    gp.public_id AS grandparent_public_id,
    gp.name      AS grandparent_name,
    (
        SELECT COUNT(*)
        FROM ownership.storage_units c
        WHERE c.parent_id = s.id
          AND c.user_id = s.user_id
          AND c.deleted_at IS NULL
    )::bigint AS child_count
FROM ownership.storage_units s
LEFT JOIN ownership.storage_units p
  ON p.id = s.parent_id
 AND p.user_id = s.user_id
LEFT JOIN ownership.storage_units gp
  ON gp.id = p.parent_id
 AND gp.user_id = s.user_id
WHERE s.user_id = @user_id
  AND s.parent_id = @parent_id
  AND s.deleted_at IS NULL
ORDER BY s.sort_order ASC, s.id ASC;

-- archive可否の判定に使用する (設計書 16章の暗黙cascade禁止方針)。
-- name: CountActiveChildStorageUnits :one
SELECT COUNT(*)
FROM ownership.storage_units
WHERE user_id = @user_id
  AND parent_id = @parent_id
  AND deleted_at IS NULL;
