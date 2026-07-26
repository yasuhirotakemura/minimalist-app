-- 全てのユーザーdata queryは user internal ID を条件に含める (設計書 11.6 / 18.3)。
--
-- 「割当数量合計 <= 所有数量」は複数行の集計を伴うためCHECK constraintで
-- 表現できない。Application Serviceがtransaction内で対象items行を
-- SELECT FOR UPDATE でロックしてから集計・検証する (設計書 20章)。

-- name: InsertStorageAllocation :one
INSERT INTO ownership.storage_allocations (
    public_id,
    user_id,
    storage_unit_id,
    item_id,
    quantity,
    created_at,
    updated_at,
    version
) VALUES (
    @public_id,
    @user_id,
    @storage_unit_id,
    @item_id,
    @quantity,
    @created_at,
    @updated_at,
    1
)
RETURNING *;

-- name: FindStorageAllocationByPublicID :one
SELECT
    sqlc.embed(a),
    i.public_id         AS item_public_id,
    i.name              AS item_name,
    i.unit_name         AS item_unit_name,
    i.quantity          AS item_quantity,
    i.weight_gram       AS item_weight_gram,
    i.volume_milliliter AS item_volume_milliliter,
    (i.deleted_at IS NOT NULL)::boolean AS item_is_archived
FROM ownership.storage_allocations a
JOIN ownership.items i
  ON i.id = a.item_id
 AND i.user_id = a.user_id
WHERE a.public_id = @public_id
  AND a.user_id = @user_id;

-- name: UpdateStorageAllocationQuantity :one
UPDATE ownership.storage_allocations
SET quantity   = @quantity,
    updated_at = @updated_at,
    version    = version + 1
WHERE public_id = @public_id
  AND user_id = @user_id
  AND version = @expected_version
RETURNING id;

-- 割当は現在状態であり取り出した履歴を保持しないため物理削除する。
-- name: DeleteStorageAllocation :one
DELETE FROM ownership.storage_allocations
WHERE public_id = @public_id
  AND user_id = @user_id
  AND version = @expected_version
RETURNING id;

-- 一括置換で既存の割当をすべて取り除く。
-- name: DeleteStorageAllocationsByStorageUnitID :exec
DELETE FROM ownership.storage_allocations
WHERE user_id = @user_id
  AND storage_unit_id = @storage_unit_id;

-- 更新件数0の理由 (不存在か競合か) を判定するために使用する。
-- name: ExistsStorageAllocationByPublicID :one
SELECT EXISTS (
    SELECT 1
    FROM ownership.storage_allocations
    WHERE public_id = @public_id
      AND user_id = @user_id
) AS exists;

-- 収納内容の一覧。複数収納単位分をまとめて取得しN+1 queryを避ける。
-- 表示順を安定させるためアイテム名昇順で返す。
-- name: ListStorageAllocationsByStorageUnitIDs :many
SELECT
    sqlc.embed(a),
    i.public_id         AS item_public_id,
    i.name              AS item_name,
    i.unit_name         AS item_unit_name,
    i.quantity          AS item_quantity,
    i.weight_gram       AS item_weight_gram,
    i.volume_milliliter AS item_volume_milliliter,
    (i.deleted_at IS NOT NULL)::boolean AS item_is_archived
FROM ownership.storage_allocations a
JOIN ownership.items i
  ON i.id = a.item_id
 AND i.user_id = a.user_id
WHERE a.user_id = @user_id
  AND a.storage_unit_id = ANY(@storage_unit_ids::bigint[])
ORDER BY i.name ASC, i.id ASC;

-- アイテム側から割当先を引く (設計書 13.14)。
-- アイテム詳細・一覧の収納情報表示と未割当数量の算出に使用する。
-- name: ListStorageAllocationsByItemIDs :many
SELECT
    sqlc.embed(a),
    s.public_id AS storage_unit_public_id,
    s.name      AS storage_unit_name
FROM ownership.storage_allocations a
JOIN ownership.storage_units s
  ON s.id = a.storage_unit_id
 AND s.user_id = a.user_id
WHERE a.user_id = @user_id
  AND a.item_id = ANY(@item_ids::bigint[])
ORDER BY s.name ASC, s.id ASC;

-- archive可否の判定に使用する。
-- name: CountStorageAllocationsByStorageUnitID :one
SELECT COUNT(*)
FROM ownership.storage_allocations
WHERE user_id = @user_id
  AND storage_unit_id = @storage_unit_id;

-- 割当数量合計。excludeAllocationIdへ0以外を渡すとその割当を合計から除く。
-- 数量変更時に「変更後の値」で再計算するために使用する。
-- name: SumStorageAllocationQuantityByItemID :one
SELECT COALESCE(SUM(quantity), 0)::bigint AS total_quantity
FROM ownership.storage_allocations
WHERE user_id = @user_id
  AND item_id = @item_id
  AND (@exclude_allocation_id::bigint = 0 OR id <> @exclude_allocation_id::bigint);

-- 一括置換で、置換対象の収納単位を除いた他収納単位への割当合計を返す。
-- name: SumStorageAllocationQuantityByItemIDsExcludingStorageUnit :many
SELECT
    item_id,
    COALESCE(SUM(quantity), 0)::bigint AS total_quantity
FROM ownership.storage_allocations
WHERE user_id = @user_id
  AND item_id = ANY(@item_ids::bigint[])
  AND storage_unit_id <> @excluded_storage_unit_id
GROUP BY item_id;

-- 対象アイテム行をロックし、並行更新で数量整合性が破られないよう直列化する
-- (設計書 20章)。deadlockを避けるため内部IDの昇順でロックする。
-- name: LockItemQuantitiesForUpdate :many
SELECT id, quantity
FROM ownership.items
WHERE user_id = @user_id
  AND id = ANY(@item_ids::bigint[])
ORDER BY id
FOR UPDATE;

-- 割当対象アイテムの解決。archive状態も返し、新規割当の可否をDomainが判断する。
-- name: FindAllocatedItemByPublicID :one
SELECT
    id,
    public_id,
    name,
    unit_name,
    quantity,
    weight_gram,
    volume_milliliter,
    (deleted_at IS NOT NULL)::boolean AS is_archived
FROM ownership.items
WHERE user_id = @user_id
  AND public_id = @public_id;

-- 一括置換で指定された複数アイテムをまとめて解決する。
-- name: ListAllocatedItemsByPublicIDs :many
SELECT
    id,
    public_id,
    name,
    unit_name,
    quantity,
    weight_gram,
    volume_milliliter,
    (deleted_at IS NOT NULL)::boolean AS is_archived
FROM ownership.items
WHERE user_id = @user_id
  AND public_id = ANY(@public_ids::uuid[]);
