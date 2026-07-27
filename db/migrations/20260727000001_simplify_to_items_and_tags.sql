-- アプリケーションのスコープを所持品とタグの管理へ縮小する。
--
-- 対象:
--   1. 収納単位・収納割当 (ownership.storage_units / ownership.storage_allocations)
--      を削除する。収納と移動管理を機能ごと廃止する。
--   2. 使用記録 (ownership.item_usage_records) と最終使用日時
--      (ownership.items.last_used_at) を削除する。
--   3. 携行区分 (ownership.items.mobility_class_code) を削除する。
--   4. 所有見直し判定・購入前審査・収納容量計算の入力としてのみ存在していた
--      ownership.items のcolumnを削除する。
--   5. 設定操作を持たない棚卸し確認 (is_confirmed / confirmed_at) を削除する。
--
-- 方針:
--   - 適用済みmigrationは書き換えず、forward-onlyで追加する。
--   - columnへ付与したCHECK制約・COMMENTはDROP COLUMNで併せて削除される。
--   - Downは削除前のschemaを復元する。dataは復元できないため、
--     column追加とtable再作成のみを行う。

-- +goose Up

-- ---------------------------------------------------------------------------
-- 1. 収納単位・収納割当
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
DROP TABLE IF EXISTS ownership.storage_allocations;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS ownership.storage_units;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- 2. 使用記録
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
DROP TABLE IF EXISTS ownership.item_usage_records;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- 3. 携行区分で絞り込むためのindex
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
DROP INDEX IF EXISTS ownership.idx_items__user_id_mobility_class_code_deleted_at;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- 4. items のcolumn
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
ALTER TABLE ownership.items
    DROP COLUMN mobility_class_code,
    DROP COLUMN last_used_at,
    DROP COLUMN substitutability_code,
    DROP COLUMN desired_quantity,
    DROP COLUMN weight_gram,
    DROP COLUMN volume_milliliter,
    DROP COLUMN is_fragile,
    DROP COLUMN is_valuable,
    DROP COLUMN is_sentimental,
    DROP COLUMN requires_maintenance,
    DROP COLUMN expires_on,
    DROP COLUMN ownership_reason,
    DROP COLUMN disposal_condition,
    DROP COLUMN purchase_amount,
    DROP COLUMN replacement_amount,
    DROP COLUMN resale_amount,
    DROP COLUMN is_confirmed,
    DROP COLUMN confirmed_at;
-- +goose StatementEnd

-- +goose Down

-- ---------------------------------------------------------------------------
-- 4. items のcolumn
--
-- NOT NULLのcolumnは既存行のためDEFAULTを付けて追加し、
-- 追加後にDEFAULTを外して元の定義へ戻す。
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
ALTER TABLE ownership.items
    ADD COLUMN mobility_class_code   TEXT        NOT NULL DEFAULT 'fixed',
    ADD COLUMN last_used_at          TIMESTAMPTZ,
    ADD COLUMN substitutability_code TEXT        NOT NULL DEFAULT 'unknown',
    ADD COLUMN desired_quantity      INTEGER,
    ADD COLUMN weight_gram           INTEGER,
    ADD COLUMN volume_milliliter     INTEGER,
    ADD COLUMN is_fragile            BOOLEAN     NOT NULL DEFAULT false,
    ADD COLUMN is_valuable           BOOLEAN     NOT NULL DEFAULT false,
    ADD COLUMN is_sentimental        BOOLEAN     NOT NULL DEFAULT false,
    ADD COLUMN requires_maintenance  BOOLEAN     NOT NULL DEFAULT false,
    ADD COLUMN expires_on            DATE,
    ADD COLUMN ownership_reason      TEXT,
    ADD COLUMN disposal_condition    TEXT,
    ADD COLUMN purchase_amount       BIGINT,
    ADD COLUMN replacement_amount    BIGINT,
    ADD COLUMN resale_amount         BIGINT,
    ADD COLUMN is_confirmed          BOOLEAN     NOT NULL DEFAULT false,
    ADD COLUMN confirmed_at          TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE ownership.items
    ALTER COLUMN mobility_class_code DROP DEFAULT,
    ALTER COLUMN substitutability_code DROP DEFAULT;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE ownership.items
    ADD CONSTRAINT ck_items__desired_quantity_not_negative
        CHECK (desired_quantity IS NULL OR desired_quantity >= 0),
    ADD CONSTRAINT ck_items__purchase_amount_not_negative
        CHECK (purchase_amount IS NULL OR purchase_amount >= 0),
    ADD CONSTRAINT ck_items__replacement_amount_not_negative
        CHECK (replacement_amount IS NULL OR replacement_amount >= 0),
    ADD CONSTRAINT ck_items__resale_amount_not_negative
        CHECK (resale_amount IS NULL OR resale_amount >= 0),
    ADD CONSTRAINT ck_items__weight_gram_not_negative
        CHECK (weight_gram IS NULL OR weight_gram >= 0),
    ADD CONSTRAINT ck_items__volume_milliliter_not_negative
        CHECK (volume_milliliter IS NULL OR volume_milliliter >= 0),
    ADD CONSTRAINT ck_items__substitutability_code_allowed
        CHECK (substitutability_code IN ('none', 'partial', 'full', 'unknown')),
    ADD CONSTRAINT ck_items__mobility_class_code_allowed
        CHECK (mobility_class_code IN
            ('worn', 'pocket', 'daily_bag', 'on_demand', 'self_carry',
             'parcel', 'mover', 'dispose_rebuy', 'fixed')),
    ADD CONSTRAINT ck_items__ownership_reason_length
        CHECK (char_length(ownership_reason) <= 1000),
    ADD CONSTRAINT ck_items__disposal_condition_length
        CHECK (char_length(disposal_condition) <= 1000),
    ADD CONSTRAINT ck_items__confirmed_at_required
        CHECK (is_confirmed = false OR confirmed_at IS NOT NULL);
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- 3. 携行区分で絞り込むためのindex
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
CREATE INDEX idx_items__user_id_mobility_class_code_deleted_at
    ON ownership.items (user_id, mobility_class_code, deleted_at);
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- 2. 使用記録
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

-- +goose StatementBegin
CREATE INDEX idx_item_usage_records__item_id_used_at
    ON ownership.item_usage_records (item_id, used_at DESC, id DESC);
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- 1. 収納単位・収納割当
--
-- COMMENTは復元しない。schemaのdata構造のみを戻す。
--
-- 以下のDDLは削除前のmigration (20260726000004) からそのまま持ち込んでいる。
-- コメント内の章番号は削除前の設計書 (7.3 / 13.8 / 13.9 / 13.14 / 16章) を指す。
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
CREATE TABLE ownership.storage_units (
    id                        BIGINT      GENERATED ALWAYS AS IDENTITY,
    public_id                 UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id                   BIGINT      NOT NULL,
    parent_id                 BIGINT,
    name                      TEXT        NOT NULL,
    storage_type_code         TEXT        NOT NULL,
    mobility_class_code       TEXT        NOT NULL,
    tare_weight_gram          INTEGER,
    maximum_weight_gram       INTEGER,
    maximum_volume_milliliter INTEGER,
    description               TEXT,
    sort_order                INTEGER     NOT NULL DEFAULT 0,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at                TIMESTAMPTZ,
    version                   INTEGER     NOT NULL DEFAULT 1,

    CONSTRAINT pk_storage_units PRIMARY KEY (id),
    CONSTRAINT uq_storage_units__public_id UNIQUE (public_id),
    -- 自己参照および storage_allocations からのcomposite foreign keyの参照先。
    CONSTRAINT uq_storage_units__user_id_id UNIQUE (user_id, id),
    CONSTRAINT fk_storage_units__user_id
        FOREIGN KEY (user_id) REFERENCES identity.users (id) ON DELETE CASCADE,
    -- 親と子が同一ユーザーに属することをDB側でも保証する (設計書 18.3)。
    -- 親削除時に子を暗黙に削除しないため ON DELETE は既定 (NO ACTION) とする。
    -- 収納単位はsoft deleteのため、この制約が働くのは物理削除時のみである。
    CONSTRAINT fk_storage_units__user_id_parent_id
        FOREIGN KEY (user_id, parent_id)
        REFERENCES ownership.storage_units (user_id, id),

    CONSTRAINT ck_storage_units__name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT ck_storage_units__name_length CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT ck_storage_units__description_length CHECK (char_length(description) <= 500),
    -- 自分自身を親に指定できない (設計書 7.3)。循環参照の検証はDomainが行う。
    CONSTRAINT ck_storage_units__parent_is_not_self
        CHECK (parent_id IS NULL OR parent_id <> id),

    CONSTRAINT ck_storage_units__storage_type_code_allowed
        CHECK (storage_type_code IN
            ('bag', 'pouch', 'box', 'shelf', 'room', 'appliance', 'other')),
    CONSTRAINT ck_storage_units__mobility_class_code_allowed
        CHECK (mobility_class_code IN
            ('worn', 'pocket', 'daily_bag', 'on_demand', 'self_carry',
             'parcel', 'mover', 'dispose_rebuy', 'fixed')),

    -- 未設定 (NULL) と 0 は意味が異なる。0は「自重を測って0gだった」ではなく
    -- 「無視できる」を表すため、利用者が明示的に入力した値として保持する。
    CONSTRAINT ck_storage_units__tare_weight_gram_not_negative
        CHECK (tare_weight_gram IS NULL OR tare_weight_gram >= 0),
    CONSTRAINT ck_storage_units__maximum_weight_gram_not_negative
        CHECK (maximum_weight_gram IS NULL OR maximum_weight_gram >= 0),
    CONSTRAINT ck_storage_units__maximum_volume_milliliter_not_negative
        CHECK (maximum_volume_milliliter IS NULL OR maximum_volume_milliliter >= 0),
    CONSTRAINT ck_storage_units__sort_order_not_negative CHECK (sort_order >= 0),
    CONSTRAINT ck_storage_units__version_positive CHECK (version > 0)
);
-- +goose StatementEnd

-- 一覧の既定sort (sort_order昇順) と、rootOnly絞り込みで使用する。
-- +goose StatementBegin
CREATE INDEX idx_storage_units__user_id_sort_order
    ON ownership.storage_units (user_id, sort_order, id)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- 子収納単位の取得、parentStorageUnitPublicIdによる絞り込み、
-- および階層の読み込みで使用する。
-- +goose StatementBegin
CREATE INDEX idx_storage_units__user_id_parent_id
    ON ownership.storage_units (user_id, parent_id)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- sort=updatedAt で使用する。
-- +goose StatementBegin
CREATE INDEX idx_storage_units__user_id_updated_at
    ON ownership.storage_units (user_id, updated_at DESC)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- storage_allocations (設計書 13.9)
--
-- 設計書 13.9 に対する追加column:
--   user_id : 他ユーザーの収納単位・アイテムを組み合わせられないことを
--             composite foreign keyで保証するため (設計書 18.3)。
--   version : 収納割当を単独で更新・削除するため楽観ロックを持たせる
--             (設計書 11.7)。設計書 13.9 は共通column方針 13.3 の
--             version を省略しているが、更新対象tableのため追加する。
--
-- 割当は「今どこに何個入っているか」を表す現在状態であり、取り出した履歴を
-- 保持する要件が無いため deleted_at を持たず物理削除する。
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
CREATE TABLE ownership.storage_allocations (
    id              BIGINT      GENERATED ALWAYS AS IDENTITY,
    public_id       UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id         BIGINT      NOT NULL,
    storage_unit_id BIGINT      NOT NULL,
    item_id         BIGINT      NOT NULL,
    quantity        INTEGER     NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    version         INTEGER     NOT NULL DEFAULT 1,

    CONSTRAINT pk_storage_allocations PRIMARY KEY (id),
    CONSTRAINT uq_storage_allocations__public_id UNIQUE (public_id),
    -- 同一収納単位・同一アイテムの組み合わせは1件とする (設計書 13.9)。
    -- 同一アイテムを複数収納単位へ分割する場合は収納単位ごとに1行となる。
    CONSTRAINT uq_storage_allocations__storage_unit_id_item_id
        UNIQUE (storage_unit_id, item_id),
    CONSTRAINT fk_storage_allocations__user_id_storage_unit_id
        FOREIGN KEY (user_id, storage_unit_id)
        REFERENCES ownership.storage_units (user_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_storage_allocations__user_id_item_id
        FOREIGN KEY (user_id, item_id)
        REFERENCES ownership.items (user_id, id) ON DELETE CASCADE,

    -- 0個の割当は「割り当てていない」と同義であり行を残す意味がない (設計書 13.9)。
    CONSTRAINT ck_storage_allocations__quantity_positive CHECK (quantity > 0),
    CONSTRAINT ck_storage_allocations__quantity_upper_bound
        CHECK (quantity <= 1000000),
    CONSTRAINT ck_storage_allocations__version_positive CHECK (version > 0)
);
-- +goose StatementEnd

-- アイテム側から割当先を引く (設計書 13.14 idx_storage_allocations__item_id)。
-- 未割当数量の算出と、アイテム詳細の収納情報表示で使用する。
-- +goose StatementBegin
CREATE INDEX idx_storage_allocations__item_id
    ON ownership.storage_allocations (item_id);
-- +goose StatementEnd

-- 収納単位の内容一覧と容量集計で使用する。
-- +goose StatementBegin
CREATE INDEX idx_storage_allocations__user_id_storage_unit_id
    ON ownership.storage_allocations (user_id, storage_unit_id);
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
