-- 収納単位・収納割当 (設計書 13.1 / 13.5 / 13.8 / 13.9 / 13.14 / 16章)
--
-- Phase 2のスコープ:
--   storage_units       : 所持品の保管場所と搬送単位
--   storage_allocations : 所持品を収納単位へ数量付きで割り当てた現在状態
--
-- 20260725000002_create_ownership_schema.sql は適用済みのため書き換えず、
-- 本migrationでownership schemaへtableを追加する (設計書 27.2 forward-only)。
--
-- 方針 (既存migrationと同一):
--   - 内部主キーは BIGINT GENERATED ALWAYS AS IDENTITY とする。
--   - 外部公開IDは public_id (UUID) とし、APIでは内部IDを公開しない。
--   - 時刻は TIMESTAMPTZ でUTC保存する。
--   - 重量・容積は0以上の整数とし、未設定はNULLで表す。0とNULLを区別する。
--   - soft delete対象は deleted_at を持つ。archiveはdeleted_atの設定として表現する。
--   - 別ユーザーのresourceを参照できないことをcomposite foreign keyで保証する。
--
-- 階層制約 (最大3階層・自己参照禁止・循環参照禁止) の分担:
--   DB   : 親が同一ユーザーであること、自分自身を親にできないこと
--   Domain: 階層上限3、循環参照禁止 (設計書 25.1 が unit test 対象と定めるため)
--   Application: 対象ユーザーの storage_units 行を SELECT FOR UPDATE でロックし、
--                並行更新で階層制約が破られないよう直列化する
--
-- 割当数量合計 <= 所有数量 の制約:
--   複数行の集計が必要でCHECK constraintでは表現できないため、
--   Application Serviceのtransaction内で対象 items 行を SELECT FOR UPDATE し、
--   合計を検証する (設計書 20章)。

-- +goose Up

-- ---------------------------------------------------------------------------
-- storage_units (設計書 13.8)
--
-- 列名は設計書 13.8 のcolumn名をそのまま使用する。
-- 重量は単数形 (tare_weight_gram) とし、items.weight_gram と揃える。
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
-- COMMENT
--
-- 業務上の意味を記述する。単なる日本語訳は書かない。
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
COMMENT ON TABLE ownership.storage_units IS
    '所持品の保管場所と移動時の搬送単位を同一概念として表す。リュック・ポーチ・箱・棚・冷蔵庫のいずれもここへ登録し、mobility_class_codeで運び方を決める。';
-- +goose StatementEnd

-- +goose StatementBegin
COMMENT ON COLUMN ownership.storage_units.parent_id IS
    '入れ子の親。ポーチをリュックへ入れるような包含関係を表す。最大3階層で、親のarchiveは子へ波及させない。';
-- +goose StatementEnd

-- +goose StatementBegin
COMMENT ON COLUMN ownership.storage_units.mobility_class_code IS
    '引っ越し・旅行時にこの収納単位ごとどう運ぶかの決定。自力搬送か宅配便か業者搬送かを収納単位単位で確定させ、搬送計画の集計キーとする。';
-- +goose StatementEnd

-- +goose StatementBegin
COMMENT ON COLUMN ownership.storage_units.tare_weight_gram IS
    '中身を入れていない状態の重量。総重量から中身の重量を切り分け、持てるかどうかの判断を可能にする。NULLは未計測を表し、集計値を不完全として扱う。';
-- +goose StatementEnd

-- +goose StatementBegin
COMMENT ON COLUMN ownership.storage_units.maximum_weight_gram IS
    '持ち運べる上限、または収納具の耐荷重。超過した状態を警告として提示するための判断基準。NULLは上限を管理しないことを表す。';
-- +goose StatementEnd

-- +goose StatementBegin
COMMENT ON COLUMN ownership.storage_units.maximum_volume_milliliter IS
    '入りきる上限容積。詰め込み過ぎを事前に検知するための判断基準。NULLは上限を管理しないことを表す。';
-- +goose StatementEnd

-- +goose StatementBegin
COMMENT ON COLUMN ownership.storage_units.deleted_at IS
    '使わなくなった収納具をarchiveした日時。誤操作から復元できるようsoft deleteとする。中身と子収納単位が残っている間はarchiveできない。';
-- +goose StatementEnd

-- +goose StatementBegin
COMMENT ON TABLE ownership.storage_allocations IS
    '所持品が今どの収納単位へ何個入っているかの現在状態。同一所持品を複数の収納単位へ分割でき、所有数量との差が未割当数量となる。';
-- +goose StatementEnd

-- +goose StatementBegin
COMMENT ON COLUMN ownership.storage_allocations.quantity IS
    'この収納単位へ入れている個数。同一アイテムの全割当の合計は所有数量以下でなければならない。差分は未割当数量として棚卸しの対象になる。';
-- +goose StatementEnd

-- +goose StatementBegin
COMMENT ON COLUMN ownership.storage_allocations.version IS
    '収納内容編集画面での同時編集を検知するための楽観ロック値。競合時は利用者の入力でserver状態を上書きしない。';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ownership.storage_allocations;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS ownership.storage_units;
-- +goose StatementEnd
