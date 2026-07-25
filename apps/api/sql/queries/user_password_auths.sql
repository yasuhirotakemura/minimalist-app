-- name: InsertUserPasswordAuth :one
INSERT INTO identity.user_password_auths (
    user_id,
    password_hash,
    algorithm,
    password_updated_at,
    created_at,
    updated_at,
    version
) VALUES (
    @user_id,
    @password_hash,
    @algorithm,
    @password_updated_at,
    @created_at,
    @updated_at,
    1
)
RETURNING id, user_id, password_hash, algorithm, password_updated_at, created_at, updated_at, version;

-- name: FindUserPasswordAuthByUserID :one
SELECT id, user_id, password_hash, algorithm, password_updated_at, created_at, updated_at, version
FROM identity.user_password_auths
WHERE user_id = @user_id;

-- name: UpdateUserPasswordAuth :execrows
UPDATE identity.user_password_auths
SET password_hash       = @password_hash,
    algorithm           = @algorithm,
    password_updated_at = @password_updated_at,
    updated_at          = @updated_at,
    version             = version + 1
WHERE user_id = @user_id
  AND version = @expected_version;
