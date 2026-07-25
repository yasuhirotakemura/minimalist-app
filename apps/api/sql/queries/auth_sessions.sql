-- session tokenは保存せず、SHA-256 hashで検索する (設計書 18.1)。

-- name: InsertAuthSession :one
INSERT INTO identity.auth_sessions (
    public_id,
    user_id,
    token_hash,
    issued_at,
    expires_at,
    last_used_at,
    user_agent,
    ip_address,
    created_at,
    updated_at
) VALUES (
    @public_id,
    @user_id,
    @token_hash,
    @issued_at,
    @expires_at,
    @last_used_at,
    @user_agent,
    @ip_address,
    @created_at,
    @updated_at
)
RETURNING id, public_id, user_id, token_hash, issued_at, expires_at, last_used_at, revoked_at, user_agent, ip_address, created_at, updated_at;

-- 有効なsessionと所有ユーザーを1回のqueryで取得する。
-- name: FindLiveAuthSessionWithUserByTokenHash :one
SELECT
    s.id           AS session_id,
    s.public_id    AS session_public_id,
    s.user_id      AS user_id,
    s.issued_at    AS issued_at,
    s.expires_at   AS expires_at,
    s.last_used_at AS last_used_at,
    u.public_id    AS user_public_id,
    u.email        AS email,
    u.display_name AS display_name,
    u.timezone     AS timezone,
    u.locale       AS locale,
    u.created_at   AS user_created_at,
    u.updated_at   AS user_updated_at,
    u.version      AS user_version
FROM identity.auth_sessions AS s
INNER JOIN identity.users AS u ON u.id = s.user_id
WHERE s.token_hash = @token_hash
  AND s.revoked_at IS NULL
  AND s.expires_at > @evaluated_at
  AND u.deleted_at IS NULL;

-- name: TouchAuthSessionLastUsedAt :execrows
UPDATE identity.auth_sessions
SET last_used_at = @last_used_at,
    updated_at   = @updated_at
WHERE token_hash = @token_hash
  AND revoked_at IS NULL;

-- name: RevokeAuthSessionByTokenHash :execrows
UPDATE identity.auth_sessions
SET revoked_at = @revoked_at,
    updated_at = @updated_at
WHERE token_hash = @token_hash
  AND revoked_at IS NULL;

-- name: RevokeAllAuthSessionsByUserID :execrows
UPDATE identity.auth_sessions
SET revoked_at = @revoked_at,
    updated_at = @updated_at
WHERE user_id = @user_id
  AND revoked_at IS NULL;

-- name: DeleteExpiredAuthSessions :execrows
DELETE FROM identity.auth_sessions
WHERE expires_at <= @expired_before;
