-- identity schema (設計書 13.1 / 13.5 / 13.6)
--
-- Phase 0のスコープは認証のみとする。
--   users               : 利用者
--   user_password_auths : password認証情報。usersから分離する (設計書 13.6)
--   auth_sessions       : session token hash
--
-- 方針:
--   - 内部主キーは BIGINT GENERATED ALWAYS AS IDENTITY とする。
--   - 外部公開IDは public_id (UUID) とし、APIでは内部IDを公開しない。
--   - 時刻は TIMESTAMPTZ でUTC保存する。
--   - 不変条件をapplicationだけでなくDB制約でも保証する。
--   - 適用済みmigrationは書き換えず、forward-onlyで追加する。

-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS identity;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE identity.users (
    id            BIGINT      GENERATED ALWAYS AS IDENTITY,
    public_id     UUID        NOT NULL DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL,
    display_name  TEXT        NOT NULL,
    timezone      TEXT        NOT NULL DEFAULT 'Asia/Tokyo',
    locale        TEXT        NOT NULL DEFAULT 'ja-JP',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,
    version       INTEGER     NOT NULL DEFAULT 1,

    CONSTRAINT pk_users PRIMARY KEY (id),
    CONSTRAINT uq_users__public_id UNIQUE (public_id),
    -- emailはlowercaseで保持する (設計書 13.6)
    CONSTRAINT ck_users__email_lowercase CHECK (email = lower(email)),
    CONSTRAINT ck_users__email_length CHECK (char_length(email) BETWEEN 3 AND 254),
    CONSTRAINT ck_users__email_shape CHECK (position('@' IN email) > 1),
    CONSTRAINT ck_users__display_name_length CHECK (char_length(display_name) BETWEEN 1 AND 100),
    CONSTRAINT ck_users__timezone_not_blank CHECK (btrim(timezone) <> ''),
    CONSTRAINT ck_users__locale_not_blank CHECK (btrim(locale) <> ''),
    CONSTRAINT ck_users__version_positive CHECK (version > 0)
);
-- +goose StatementEnd

-- soft delete済みのemailは再登録できるようにするため、部分unique indexとする。
-- +goose StatementBegin
CREATE UNIQUE INDEX uq_users__email_active
    ON identity.users (email)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- password認証情報。APIへ公開しないためpublic_idを持たない。
-- +goose StatementBegin
CREATE TABLE identity.user_password_auths (
    id                  BIGINT      GENERATED ALWAYS AS IDENTITY,
    user_id             BIGINT      NOT NULL,
    -- Argon2idのPHC文字列 ($argon2id$v=19$m=...,t=...,p=...$salt$hash)
    password_hash       TEXT        NOT NULL,
    algorithm           TEXT        NOT NULL DEFAULT 'argon2id',
    password_updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    version             INTEGER     NOT NULL DEFAULT 1,

    CONSTRAINT pk_user_password_auths PRIMARY KEY (id),
    CONSTRAINT uq_user_password_auths__user_id UNIQUE (user_id),
    CONSTRAINT fk_user_password_auths__user_id
        FOREIGN KEY (user_id) REFERENCES identity.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_user_password_auths__password_hash_shape
        CHECK (password_hash LIKE '$argon2id$%'),
    CONSTRAINT ck_user_password_auths__algorithm_allowed
        CHECK (algorithm IN ('argon2id')),
    CONSTRAINT ck_user_password_auths__version_positive CHECK (version > 0)
);
-- +goose StatementEnd

-- session token本体は保存せず、SHA-256 hashのみを保存する (設計書 18.1)。
-- 楽観ロック対象ではないためversionを持たない。
-- +goose StatementBegin
CREATE TABLE identity.auth_sessions (
    id           BIGINT      GENERATED ALWAYS AS IDENTITY,
    public_id    UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id      BIGINT      NOT NULL,
    token_hash   BYTEA       NOT NULL,
    issued_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at   TIMESTAMPTZ,
    user_agent   TEXT,
    ip_address   INET,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT pk_auth_sessions PRIMARY KEY (id),
    CONSTRAINT uq_auth_sessions__public_id UNIQUE (public_id),
    CONSTRAINT uq_auth_sessions__token_hash UNIQUE (token_hash),
    CONSTRAINT fk_auth_sessions__user_id
        FOREIGN KEY (user_id) REFERENCES identity.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_auth_sessions__token_hash_length CHECK (octet_length(token_hash) = 32),
    CONSTRAINT ck_auth_sessions__expires_after_issued CHECK (expires_at > issued_at),
    CONSTRAINT ck_auth_sessions__user_agent_length CHECK (char_length(user_agent) <= 512)
);
-- +goose StatementEnd

-- session一覧・期限切れ削除で使用する。
-- +goose StatementBegin
CREATE INDEX idx_auth_sessions__user_id_expires_at
    ON identity.auth_sessions (user_id, expires_at DESC);
-- +goose StatementEnd

-- 期限切れsessionの掃除で使用する。
-- +goose StatementBegin
CREATE INDEX idx_auth_sessions__expires_at
    ON identity.auth_sessions (expires_at)
    WHERE revoked_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS identity.auth_sessions;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS identity.user_password_auths;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS identity.users;
-- +goose StatementEnd

-- +goose StatementBegin
DROP SCHEMA IF EXISTS identity;
-- +goose StatementEnd
