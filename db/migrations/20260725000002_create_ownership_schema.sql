-- ownership schema (設計書 13.1 / 13.5 / 13.7 / 13.14)
--
-- Phase 1のスコープ:
--   categories          : アイテムの主分類
--   tags                : 任意のlabel
--   items               : 所持品と所有判断情報
--   item_tags           : itemsとtagsの関連
--   item_usage_records  : 使用記録
--
-- storage_units / storage_allocations は設計書 13.5 では本schemaに属するが、
-- Phase 2のスコープのため本migrationでは作成しない。
--
-- 方針:
--   - 内部主キーは BIGINT GENERATED ALWAYS AS IDENTITY とする。
--   - 外部公開IDは public_id (UUID) とし、APIでは内部IDを公開しない。
--   - 時刻は TIMESTAMPTZ でUTC保存する。
--   - 金額は円単位の整数で保持し、浮動小数点数を使用しない (設計書 11章)。
--   - soft delete対象は deleted_at を持つ。archiveはdeleted_atの設定として表現する。
--   - 不変条件をapplicationだけでなくDB制約でも保証する (設計書 4.3)。
--   - 別ユーザーのresourceを参照できないことをcomposite foreign keyで保証する。
--     そのため親tableへ UNIQUE (user_id, id) を置き、子tableは user_id を併せて持つ。
--   - 適用済みmigrationは書き換えず、forward-onlyで追加する。

-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS ownership;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- categories
--
-- 列定義は設計書 13.5 でtable名のみが示されているため、
-- 13.3 の共通column と 12.4 のendpoint (一覧・表示順) から決定した。
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
CREATE TABLE ownership.categories (
    id          BIGINT      GENERATED ALWAYS AS IDENTITY,
    public_id   UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id     BIGINT      NOT NULL,
    name        TEXT        NOT NULL,
    description TEXT,
    sort_order  INTEGER     NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    version     INTEGER     NOT NULL DEFAULT 1,

    CONSTRAINT pk_categories PRIMARY KEY (id),
    CONSTRAINT uq_categories__public_id UNIQUE (public_id),
    -- itemsからのcomposite foreign keyの参照先として使用する。
    CONSTRAINT uq_categories__user_id_id UNIQUE (user_id, id),
    CONSTRAINT fk_categories__user_id
        FOREIGN KEY (user_id) REFERENCES identity.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_categories__name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT ck_categories__name_length CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT ck_categories__description_length CHECK (char_length(description) <= 500),
    CONSTRAINT ck_categories__sort_order_not_negative CHECK (sort_order >= 0),
    CONSTRAINT ck_categories__version_positive CHECK (version > 0)
);
-- +goose StatementEnd

-- 有効なカテゴリー名はユーザー内で一意とする。削除済みの名称は再利用できる。
-- +goose StatementBegin
CREATE UNIQUE INDEX uq_categories__user_id_name_active
    ON ownership.categories (user_id, name)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- 一覧はsort_order昇順で取得する。
-- +goose StatementBegin
CREATE INDEX idx_categories__user_id_sort_order
    ON ownership.categories (user_id, sort_order, id)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- tags
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
CREATE TABLE ownership.tags (
    id         BIGINT      GENERATED ALWAYS AS IDENTITY,
    public_id  UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id    BIGINT      NOT NULL,
    name       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    version    INTEGER     NOT NULL DEFAULT 1,

    CONSTRAINT pk_tags PRIMARY KEY (id),
    CONSTRAINT uq_tags__public_id UNIQUE (public_id),
    -- item_tagsからのcomposite foreign keyの参照先として使用する。
    CONSTRAINT uq_tags__user_id_id UNIQUE (user_id, id),
    CONSTRAINT fk_tags__user_id
        FOREIGN KEY (user_id) REFERENCES identity.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_tags__name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT ck_tags__name_length CHECK (char_length(name) BETWEEN 1 AND 50),
    CONSTRAINT ck_tags__version_positive CHECK (version > 0)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE UNIQUE INDEX uq_tags__user_id_name_active
    ON ownership.tags (user_id, name)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_tags__user_id_name
    ON ownership.tags (user_id, name)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- items (設計書 13.7)
--
-- enumの値集合:
--   item_kind_code        : 設計書に値集合の定義が無い。12.5の例 durable を基に2値で開始する。
--   necessity_level_code  : 設計書 14.5
--   usage_frequency_code  : 設計書 14.3
--   substitutability_code : 設計書 14.4
--   mobility_class_code   : 設計書 16.1
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
CREATE TABLE ownership.items (
    id                    BIGINT      GENERATED ALWAYS AS IDENTITY,
    public_id             UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id               BIGINT      NOT NULL,
    category_id           BIGINT      NOT NULL,
    name                  TEXT        NOT NULL,
    item_kind_code        TEXT        NOT NULL,
    quantity              INTEGER     NOT NULL,
    desired_quantity      INTEGER,
    unit_name             TEXT        NOT NULL,
    necessity_level_code  TEXT        NOT NULL,
    usage_frequency_code  TEXT        NOT NULL,
    substitutability_code TEXT        NOT NULL,
    mobility_class_code   TEXT        NOT NULL,
    ownership_reason      TEXT,
    disposal_condition    TEXT,
    last_used_at          TIMESTAMPTZ,
    purchased_on          DATE,
    purchase_amount       BIGINT,
    replacement_amount    BIGINT,
    resale_amount         BIGINT,
    weight_gram           INTEGER,
    volume_milliliter     INTEGER,
    is_fragile            BOOLEAN     NOT NULL DEFAULT false,
    is_valuable           BOOLEAN     NOT NULL DEFAULT false,
    is_sentimental        BOOLEAN     NOT NULL DEFAULT false,
    requires_maintenance  BOOLEAN     NOT NULL DEFAULT false,
    expires_on            DATE,
    source_url            TEXT,
    notes                 TEXT,
    is_confirmed          BOOLEAN     NOT NULL DEFAULT false,
    confirmed_at          TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at            TIMESTAMPTZ,
    version               INTEGER     NOT NULL DEFAULT 1,

    CONSTRAINT pk_items PRIMARY KEY (id),
    CONSTRAINT uq_items__public_id UNIQUE (public_id),
    -- item_tags / item_usage_records からのcomposite foreign keyの参照先。
    CONSTRAINT uq_items__user_id_id UNIQUE (user_id, id),
    CONSTRAINT fk_items__user_id
        FOREIGN KEY (user_id) REFERENCES identity.users (id) ON DELETE CASCADE,
    -- 他ユーザーのカテゴリーを参照できないことをDB側でも保証する (設計書 18.3)。
    CONSTRAINT fk_items__user_id_category_id
        FOREIGN KEY (user_id, category_id)
        REFERENCES ownership.categories (user_id, id),

    CONSTRAINT ck_items__name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT ck_items__name_length CHECK (char_length(name) BETWEEN 1 AND 200),
    CONSTRAINT ck_items__unit_name_not_blank CHECK (btrim(unit_name) <> ''),
    CONSTRAINT ck_items__unit_name_length CHECK (char_length(unit_name) BETWEEN 1 AND 20),

    CONSTRAINT ck_items__quantity_not_negative CHECK (quantity >= 0),
    CONSTRAINT ck_items__desired_quantity_not_negative
        CHECK (desired_quantity IS NULL OR desired_quantity >= 0),
    CONSTRAINT ck_items__purchase_amount_not_negative
        CHECK (purchase_amount IS NULL OR purchase_amount >= 0),
    CONSTRAINT ck_items__replacement_amount_not_negative
        CHECK (replacement_amount IS NULL OR replacement_amount >= 0),
    CONSTRAINT ck_items__resale_amount_not_negative
        CHECK (resale_amount IS NULL OR resale_amount >= 0),
    CONSTRAINT ck_items__weight_gram_not_negative
        CHECK (weight_gram IS NULL OR weight_gram >= 0),
    CONSTRAINT ck_items__volume_milliliter_not_negative
        CHECK (volume_milliliter IS NULL OR volume_milliliter >= 0),

    CONSTRAINT ck_items__item_kind_code_allowed
        CHECK (item_kind_code IN ('durable', 'consumable')),
    CONSTRAINT ck_items__necessity_level_code_allowed
        CHECK (necessity_level_code IN
            ('essential', 'important', 'optional', 'undecided', 'unnecessary')),
    CONSTRAINT ck_items__usage_frequency_code_allowed
        CHECK (usage_frequency_code IN
            ('daily', 'weekly', 'monthly', 'quarterly', 'yearly', 'rarely', 'never')),
    CONSTRAINT ck_items__substitutability_code_allowed
        CHECK (substitutability_code IN ('none', 'partial', 'full', 'unknown')),
    CONSTRAINT ck_items__mobility_class_code_allowed
        CHECK (mobility_class_code IN
            ('worn', 'pocket', 'daily_bag', 'on_demand', 'self_carry',
             'parcel', 'mover', 'dispose_rebuy', 'fixed')),

    CONSTRAINT ck_items__ownership_reason_length CHECK (char_length(ownership_reason) <= 1000),
    CONSTRAINT ck_items__disposal_condition_length CHECK (char_length(disposal_condition) <= 1000),
    CONSTRAINT ck_items__notes_length CHECK (char_length(notes) <= 2000),
    -- URL schemeをhttp/httpsへ限定する (設計書 24.15)。
    CONSTRAINT ck_items__source_url_scheme
        CHECK (source_url IS NULL OR source_url ~ '^https?://'),
    CONSTRAINT ck_items__source_url_length CHECK (char_length(source_url) <= 2048),

    -- 確認済みの場合は確認日時を必須とする (設計書 13.7)。
    CONSTRAINT ck_items__confirmed_at_required
        CHECK (is_confirmed = false OR confirmed_at IS NOT NULL),
    CONSTRAINT ck_items__version_positive CHECK (version > 0)
);
-- +goose StatementEnd

-- 設計書 13.14 のindex。実際のqueryに対応させる。
-- +goose StatementBegin
CREATE INDEX idx_items__user_id_deleted_at
    ON ownership.items (user_id, deleted_at);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_items__user_id_category_id_deleted_at
    ON ownership.items (user_id, category_id, deleted_at);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_items__user_id_mobility_class_code_deleted_at
    ON ownership.items (user_id, mobility_class_code, deleted_at);
-- +goose StatementEnd

-- 既定のsort (updated_at降順) で使用する。
-- +goose StatementBegin
CREATE INDEX idx_items__user_id_updated_at
    ON ownership.items (user_id, updated_at DESC)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- item_tags
--
-- 設計書 13.2 のER図はitemsとtagsのみを関連付けるが、
-- 別ユーザーのtagを付与できないことをDB側でも保証するため user_id を持たせ、
-- composite foreign keyで参照する。
-- 付与情報は不変 (作成と削除のみ) のため updated_at / version を持たない。
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
CREATE TABLE ownership.item_tags (
    user_id    BIGINT      NOT NULL,
    item_id    BIGINT      NOT NULL,
    tag_id     BIGINT      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT pk_item_tags PRIMARY KEY (item_id, tag_id),
    CONSTRAINT fk_item_tags__user_id_item_id
        FOREIGN KEY (user_id, item_id)
        REFERENCES ownership.items (user_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_item_tags__user_id_tag_id
        FOREIGN KEY (user_id, tag_id)
        REFERENCES ownership.tags (user_id, id) ON DELETE CASCADE
);
-- +goose StatementEnd

-- タグによるアイテム絞り込みで使用する。
-- +goose StatementBegin
CREATE INDEX idx_item_tags__tag_id_item_id
    ON ownership.item_tags (tag_id, item_id);
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- item_usage_records
--
-- 追記のみのtableのため updated_at / deleted_at / version を持たない。
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
CREATE TABLE ownership.item_usage_records (
    id         BIGINT      GENERATED ALWAYS AS IDENTITY,
    public_id  UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id    BIGINT      NOT NULL,
    item_id    BIGINT      NOT NULL,
    used_at    TIMESTAMPTZ NOT NULL,
    quantity   INTEGER     NOT NULL DEFAULT 1,
    note       TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT pk_item_usage_records PRIMARY KEY (id),
    CONSTRAINT uq_item_usage_records__public_id UNIQUE (public_id),
    CONSTRAINT fk_item_usage_records__user_id_item_id
        FOREIGN KEY (user_id, item_id)
        REFERENCES ownership.items (user_id, id) ON DELETE CASCADE,
    CONSTRAINT ck_item_usage_records__quantity_positive CHECK (quantity > 0),
    CONSTRAINT ck_item_usage_records__note_length CHECK (char_length(note) <= 500)
);
-- +goose StatementEnd

-- 履歴は使用日時の降順で表示する。
-- +goose StatementBegin
CREATE INDEX idx_item_usage_records__item_id_used_at
    ON ownership.item_usage_records (item_id, used_at DESC, id DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ownership.item_usage_records;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS ownership.item_tags;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS ownership.items;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS ownership.tags;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS ownership.categories;
-- +goose StatementEnd

-- +goose StatementBegin
DROP SCHEMA IF EXISTS ownership;
-- +goose StatementEnd
