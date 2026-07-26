-- 全てのユーザーdata queryは user internal ID を条件に含める (設計書 11.6 / 18.3)。

-- name: InsertCategory :one
INSERT INTO ownership.categories (
    public_id,
    user_id,
    name,
    description,
    sort_order,
    created_at,
    updated_at,
    version
) VALUES (
    @public_id,
    @user_id,
    @name,
    @description,
    @sort_order,
    @created_at,
    @updated_at,
    1
)
RETURNING *;

-- name: ListActiveCategoriesByUserID :many
SELECT *
FROM ownership.categories
WHERE user_id = @user_id
  AND deleted_at IS NULL
ORDER BY sort_order, id;

-- name: FindActiveCategoryByPublicID :one
SELECT *
FROM ownership.categories
WHERE public_id = @public_id
  AND user_id = @user_id
  AND deleted_at IS NULL;
