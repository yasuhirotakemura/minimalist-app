-- 全てのユーザーdata queryは user internal ID もしくは一意な公開値を条件に含める (設計書 11.6 / 18.3)。

-- name: InsertUser :one
INSERT INTO identity.users (
    public_id,
    email,
    display_name,
    timezone,
    locale,
    created_at,
    updated_at,
    version
) VALUES (
    @public_id,
    @email,
    @display_name,
    @timezone,
    @locale,
    @created_at,
    @updated_at,
    1
)
RETURNING id, public_id, email, display_name, timezone, locale, created_at, updated_at, deleted_at, version;

-- name: FindActiveUserByEmail :one
SELECT id, public_id, email, display_name, timezone, locale, created_at, updated_at, deleted_at, version
FROM identity.users
WHERE email = @email
  AND deleted_at IS NULL;

-- name: FindActiveUserByPublicID :one
SELECT id, public_id, email, display_name, timezone, locale, created_at, updated_at, deleted_at, version
FROM identity.users
WHERE public_id = @public_id
  AND deleted_at IS NULL;

-- name: FindActiveUserByID :one
SELECT id, public_id, email, display_name, timezone, locale, created_at, updated_at, deleted_at, version
FROM identity.users
WHERE id = @id
  AND deleted_at IS NULL;

-- name: ExistsActiveUserByEmail :one
SELECT EXISTS (
    SELECT 1
    FROM identity.users
    WHERE email = @email
      AND deleted_at IS NULL
) AS exists;
