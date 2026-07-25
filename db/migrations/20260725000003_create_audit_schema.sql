-- audit schema (設計書 13.1 / 13.5 / 13.14 / 22章)
--
-- 記録対象は設計書 22章に定める操作とする。
-- Phase 1では以下を記録する。
--   default_categories_created / item_created / item_updated / item_archived /
--   item_restored / item_usage_recorded / tag_created / tag_updated / tag_deleted
--
-- 方針:
--   - 追記のみのtableとする。更新・削除を行わないため updated_at / version / deleted_at を持たない。
--   - changesは差分のみを保存し、機微情報 (password、session token、CSRF secret等) を保存しない。
--   - action_code / target_type_code は phase ごとに増えるため、値集合ではなく
--     形式 (lowercase snake_case) をCHECKで保証する。

-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS audit;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE audit.audit_logs (
    id               BIGINT      GENERATED ALWAYS AS IDENTITY,
    public_id        UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id          BIGINT      NOT NULL,
    action_code      TEXT        NOT NULL,
    target_type_code TEXT        NOT NULL,
    -- 対象resourceの外部公開ID。対象が単一resourceでない操作ではNULLとする。
    target_public_id UUID,
    -- 差分のみを保持する表示用payload。検索条件には使用しない。
    changes          JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT pk_audit_logs PRIMARY KEY (id),
    CONSTRAINT uq_audit_logs__public_id UNIQUE (public_id),
    CONSTRAINT fk_audit_logs__user_id
        FOREIGN KEY (user_id) REFERENCES identity.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_audit_logs__action_code_shape
        CHECK (action_code ~ '^[a-z][a-z0-9_]{0,62}$'),
    CONSTRAINT ck_audit_logs__target_type_code_shape
        CHECK (target_type_code ~ '^[a-z][a-z0-9_]{0,31}$'),
    CONSTRAINT ck_audit_logs__changes_is_object
        CHECK (jsonb_typeof(changes) = 'object')
);
-- +goose StatementEnd

-- 操作履歴画面はユーザー単位で新しい順に表示する (設計書 13.14)。
-- +goose StatementBegin
CREATE INDEX idx_audit_logs__user_id_created_at
    ON audit.audit_logs (user_id, created_at DESC, id DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit.audit_logs;
-- +goose StatementEnd

-- +goose StatementBegin
DROP SCHEMA IF EXISTS audit;
-- +goose StatementEnd
