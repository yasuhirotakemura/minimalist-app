//go:build integration

package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
	infrapostgresql "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/postgresql"
	repositories "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/repositories/postgresql"
)

// storageRepositoryFixture はrepository testで使う実体をまとめる。
type storageRepositoryFixture struct {
	units       *repositories.PostgresqlStorageUnitRepository
	allocations *repositories.PostgresqlStorageAllocationRepository
	ownerID     domainauth.UserID
	intruderID  domainauth.UserID
	categoryID  int64
}

func newStorageRepositoryFixture(t *testing.T) storageRepositoryFixture {
	t.Helper()
	truncateAll(t)

	ownerID := insertRawUser(t, "storage-repo-owner@example.com")
	intruderID := insertRawUser(t, "storage-repo-intruder@example.com")

	return storageRepositoryFixture{
		units:       repositories.NewPostgresqlStorageUnitRepository(testPool),
		allocations: repositories.NewPostgresqlStorageAllocationRepository(testPool),
		ownerID:     ownerID,
		intruderID:  intruderID,
		categoryID:  insertRawCategory(t, ownerID, "外出・携行品"),
	}
}

// insertRawUser はrepository testのためにSQLで直接ユーザーを作成する。
func insertRawUser(t *testing.T, email string) domainauth.UserID {
	t.Helper()

	var userID int64
	err := testPool.QueryRow(
		t.Context(),
		`INSERT INTO identity.users
		     (public_id, email, display_name, timezone, locale, created_at, updated_at, version)
		 VALUES (gen_random_uuid(), $1, $1, 'Asia/Tokyo', 'ja-JP', now(), now(), 1)
		 RETURNING id`,
		email,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("insert user %s: %v", email, err)
	}
	return domainauth.UserID(userID)
}

func insertRawCategory(t *testing.T, userID domainauth.UserID, name string) int64 {
	t.Helper()

	var categoryID int64
	err := testPool.QueryRow(
		t.Context(),
		`INSERT INTO ownership.categories
		     (public_id, user_id, name, sort_order, created_at, updated_at, version)
		 VALUES (gen_random_uuid(), $1, $2, 10, now(), now(), 1)
		 RETURNING id`,
		userID.Int64(), name,
	).Scan(&categoryID)
	if err != nil {
		t.Fatalf("insert category %s: %v", name, err)
	}
	return categoryID
}

// insertRawItem はrepository testのためにSQLで直接アイテムを作成する。
func (f storageRepositoryFixture) insertRawItem(
	t *testing.T, name string, quantity int32, weightGram *int32,
) domainstorage.AllocatedItem {
	t.Helper()

	var itemID int64
	var publicID uuid.UUID
	err := testPool.QueryRow(
		t.Context(),
		`INSERT INTO ownership.items
		     (public_id, user_id, category_id, name, item_kind_code, quantity, unit_name,
		      necessity_level_code, usage_frequency_code, substitutability_code,
		      mobility_class_code, weight_gram, created_at, updated_at, version)
		 VALUES (gen_random_uuid(), $1, $2, $3, 'durable', $4, '個',
		         'essential', 'monthly', 'none', 'daily_bag', $5, now(), now(), 1)
		 RETURNING id, public_id`,
		f.ownerID.Int64(), f.categoryID, name, quantity, weightGram,
	).Scan(&itemID, &publicID)
	if err != nil {
		t.Fatalf("insert item %s: %v", name, err)
	}

	return domainstorage.AllocatedItem{
		ID:         domainitem.ItemID(itemID),
		PublicID:   publicID,
		Name:       name,
		UnitName:   "個",
		Quantity:   quantity,
		WeightGram: weightGram,
	}
}

// newUnit は収納単位を1件作成する。
func (f storageRepositoryFixture) newUnit(
	t *testing.T,
	userID domainauth.UserID,
	name string,
	parent *domainstorage.StorageUnit,
	sortOrder int32,
) domainstorage.StorageUnit {
	t.Helper()

	attributes := domainstorage.Attributes{
		Name:           name,
		StorageType:    domainstorage.StorageTypeBag,
		MobilityClass:  domainitem.MobilityClassDailyBag,
		TareWeightGram: pointerTo(int32(500)),
		SortOrder:      sortOrder,
	}

	unit, err := domainstorage.NewStorageUnit(
		uuid.New(), userID, attributes, parent, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewStorageUnit(%s): %v", name, err)
	}

	created, err := f.units.Create(t.Context(), unit)
	if err != nil {
		t.Fatalf("Create(%s): %v", name, err)
	}
	return created
}

func pointerTo[T any](value T) *T { return &value }

func TestStorageUnitRepository_publicIdで取得しuserIdで分離する(t *testing.T) {
	fixture := newStorageRepositoryFixture(t)

	unit := fixture.newUnit(t, fixture.ownerID, "日常リュック", nil, 10)

	found, err := fixture.units.FindByPublicID(t.Context(), fixture.ownerID, unit.PublicID())
	if err != nil {
		t.Fatalf("FindByPublicID: %v", err)
	}
	if found.Name() != "日常リュック" {
		t.Errorf("Name = %q, want 日常リュック", found.Name())
	}

	// 他ユーザーからは存在しない扱いとする (設計書 18.3)。
	_, err = fixture.units.FindByPublicID(t.Context(), fixture.intruderID, unit.PublicID())
	if !errors.Is(err, domainstorage.ErrStorageUnitNotFound) {
		t.Fatalf("error = %v, want ErrStorageUnitNotFound", err)
	}
}

func TestStorageUnitRepository_階層を取得する(t *testing.T) {
	fixture := newStorageRepositoryFixture(t)

	root := fixture.newUnit(t, fixture.ownerID, "部屋", nil, 10)
	middle := fixture.newUnit(t, fixture.ownerID, "日常リュック", &root, 20)
	leaf := fixture.newUnit(t, fixture.ownerID, "ガジェットポーチ", &middle, 30)

	found, err := fixture.units.FindByPublicID(t.Context(), fixture.ownerID, leaf.PublicID())
	if err != nil {
		t.Fatalf("FindByPublicID: %v", err)
	}
	if found.Depth() != 3 {
		t.Errorf("Depth = %d, want 3", found.Depth())
	}
	ancestors := found.Ancestors()
	if len(ancestors) != 2 ||
		ancestors[0].ID != root.ID() || ancestors[1].ID != middle.ID() {
		t.Errorf("Ancestors = %+v, want [root, middle]", ancestors)
	}

	children, err := fixture.units.ListChildren(t.Context(), fixture.ownerID, root.ID())
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if len(children) != 1 || children[0].ID() != middle.ID() {
		t.Errorf("children = %+v, want [middle]", children)
	}

	count, err := fixture.units.CountActiveChildren(t.Context(), fixture.ownerID, root.ID())
	if err != nil {
		t.Fatalf("CountActiveChildren: %v", err)
	}
	if count != 1 {
		t.Errorf("CountActiveChildren = %d, want 1", count)
	}
}

func TestStorageUnitRepository_soft_deleteとfilterとpagination(t *testing.T) {
	fixture := newStorageRepositoryFixture(t)

	first := fixture.newUnit(t, fixture.ownerID, "収納A", nil, 10)
	fixture.newUnit(t, fixture.ownerID, "収納B", nil, 20)
	fixture.newUnit(t, fixture.ownerID, "収納C", nil, 30)
	// 他ユーザーの収納単位は一覧へ現れない。
	fixture.newUnit(t, fixture.intruderID, "他人の収納", nil, 10)

	if _, err := fixture.units.Archive(
		t.Context(), fixture.ownerID, first.PublicID(), first.Version(), time.Now().UTC(),
	); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	criteria, err := domainstorage.NewListCriteria(domainstorage.ListCriteriaInput{})
	if err != nil {
		t.Fatalf("NewListCriteria: %v", err)
	}

	units, err := fixture.units.List(t.Context(), fixture.ownerID, criteria)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(units) != 2 {
		t.Errorf("List = %d, want 2 (archive済みを除外)", len(units))
	}

	count, err := fixture.units.Count(t.Context(), fixture.ownerID, criteria)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("Count = %d, want 2", count)
	}

	withArchived, err := domainstorage.NewListCriteria(
		domainstorage.ListCriteriaInput{IncludeArchived: true})
	if err != nil {
		t.Fatalf("NewListCriteria: %v", err)
	}
	units, err = fixture.units.List(t.Context(), fixture.ownerID, withArchived)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(units) != 3 {
		t.Errorf("List(includeArchived) = %d, want 3", len(units))
	}

	// pagination
	paged, err := domainstorage.NewListCriteria(domainstorage.ListCriteriaInput{
		Limit: pointerTo(int32(1)), Offset: pointerTo(int32(1)),
	})
	if err != nil {
		t.Fatalf("NewListCriteria: %v", err)
	}
	units, err = fixture.units.List(t.Context(), fixture.ownerID, paged)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(units) != 1 || units[0].Name() != "収納C" {
		t.Errorf("paged = %+v, want [収納C]", units)
	}
}

func TestStorageUnitRepository_versionが一致しない更新は競合になる(t *testing.T) {
	fixture := newStorageRepositoryFixture(t)

	unit := fixture.newUnit(t, fixture.ownerID, "日常リュック", nil, 10)

	attributes := unit.Attributes()
	attributes.Name = "通勤リュック"
	next, err := unit.Update(attributes, nil, 1, unit.Version(), time.Now().UTC())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if _, err := fixture.units.Update(t.Context(), next, unit.Version()); err != nil {
		t.Fatalf("repository Update: %v", err)
	}

	// 同じversionでの再更新は競合となる。
	_, err = fixture.units.Update(t.Context(), next, unit.Version())
	if !errors.Is(err, domainstorage.ErrStorageUnitVersionConflict) {
		t.Fatalf("error = %v, want ErrStorageUnitVersionConflict", err)
	}
}

func TestStorageAllocationRepository_内容一覧とアイテム側からの参照(t *testing.T) {
	fixture := newStorageRepositoryFixture(t)

	bag := fixture.newUnit(t, fixture.ownerID, "衣服圧縮バッグ", nil, 10)
	backpack := fixture.newUnit(t, fixture.ownerID, "日常リュック", nil, 20)
	shirt := fixture.insertRawItem(t, "半袖シャツ", 3, pointerTo(int32(150)))

	mustCreateAllocation(t, fixture, bag, shirt, 2)
	mustCreateAllocation(t, fixture, backpack, shirt, 1)

	byUnit, err := fixture.allocations.ListByStorageUnitID(
		t.Context(), fixture.ownerID, bag.ID())
	if err != nil {
		t.Fatalf("ListByStorageUnitID: %v", err)
	}
	if len(byUnit) != 1 || byUnit[0].Quantity() != 2 {
		t.Errorf("byUnit = %+v, want 1件 quantity=2", byUnit)
	}
	if byUnit[0].Item().Name != "半袖シャツ" {
		t.Errorf("Item.Name = %q, want 半袖シャツ", byUnit[0].Item().Name)
	}

	byItem, err := fixture.allocations.ListByItemID(t.Context(), fixture.ownerID, shirt.ID)
	if err != nil {
		t.Fatalf("ListByItemID: %v", err)
	}
	if len(byItem) != 2 {
		t.Fatalf("byItem = %d, want 2", len(byItem))
	}
	// アイテム側から見た場合は収納単位名を持つ。
	if byItem[0].StorageUnit().Name == "" {
		t.Error("StorageUnit.Name が空である")
	}

	// 他ユーザーからは見えない。
	byItem, err = fixture.allocations.ListByItemID(t.Context(), fixture.intruderID, shirt.ID)
	if err != nil {
		t.Fatalf("ListByItemID(intruder): %v", err)
	}
	if len(byItem) != 0 {
		t.Errorf("intruder byItem = %d, want 0", len(byItem))
	}
}

func TestStorageAllocationRepository_同一収納単位への重複はunique違反になる(t *testing.T) {
	fixture := newStorageRepositoryFixture(t)

	bag := fixture.newUnit(t, fixture.ownerID, "衣服圧縮バッグ", nil, 10)
	shirt := fixture.insertRawItem(t, "半袖シャツ", 5, nil)

	mustCreateAllocation(t, fixture, bag, shirt, 1)

	allocation, err := domainstorage.NewStorageAllocation(
		uuid.New(), fixture.ownerID, bag, shirt, 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewStorageAllocation: %v", err)
	}
	_, err = fixture.allocations.Create(t.Context(), allocation)
	if !errors.Is(err, domainstorage.ErrStorageAllocationAlreadyExists) {
		t.Fatalf("error = %v, want ErrStorageAllocationAlreadyExists", err)
	}
}

func TestStorageAllocationRepository_versionが一致しない削除は競合になる(t *testing.T) {
	fixture := newStorageRepositoryFixture(t)

	bag := fixture.newUnit(t, fixture.ownerID, "衣服圧縮バッグ", nil, 10)
	shirt := fixture.insertRawItem(t, "半袖シャツ", 5, nil)
	created := mustCreateAllocation(t, fixture, bag, shirt, 1)

	err := fixture.allocations.Delete(
		t.Context(), fixture.ownerID, created.PublicID(), created.Version()+1)
	if !errors.Is(err, domainstorage.ErrStorageAllocationVersionConflict) {
		t.Fatalf("error = %v, want ErrStorageAllocationVersionConflict", err)
	}

	if err := fixture.allocations.Delete(
		t.Context(), fixture.ownerID, created.PublicID(), created.Version()); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

// TestStorageAllocationRepository_SELECT_FOR_UPDATEが数量整合性を守る は、
// 2つのtransactionが同じアイテムへ同時に割り当てても
// 「割当数量合計 <= 所有数量」が破られないことを確認する (設計書 20章)。
func TestStorageAllocationRepository_SELECT_FOR_UPDATEが数量整合性を守る(t *testing.T) {
	fixture := newStorageRepositoryFixture(t)

	first := fixture.newUnit(t, fixture.ownerID, "収納A", nil, 10)
	second := fixture.newUnit(t, fixture.ownerID, "収納B", nil, 20)
	// 所有数量1に対して、2つのtransactionが同時に1個ずつ割り当てようとする。
	shirt := fixture.insertRawItem(t, "半袖シャツ", 1, nil)

	transactionManager := infrapostgresql.NewTransactionManager(testPool)

	var waitGroup sync.WaitGroup
	results := make([]error, 2)
	start := make(chan struct{})

	for index, unit := range []domainstorage.StorageUnit{first, second} {
		waitGroup.Add(1)
		go func(index int, unit domainstorage.StorageUnit) {
			defer waitGroup.Done()
			<-start

			results[index] = transactionManager.WithinTransaction(
				context.Background(),
				func(ctx context.Context) error {
					ownedQuantity, allocatedQuantity, err :=
						fixture.allocations.SumQuantityByItemIDForUpdate(
							ctx, fixture.ownerID, shirt.ID, 0)
					if err != nil {
						return err
					}
					if err := domainstorage.EnsureAllocatedQuantityWithinOwned(
						ownedQuantity, allocatedQuantity+1); err != nil {
						return err
					}

					allocation, err := domainstorage.NewStorageAllocation(
						uuid.New(), fixture.ownerID, unit, shirt, 1, time.Now().UTC())
					if err != nil {
						return err
					}
					_, err = fixture.allocations.Create(ctx, allocation)
					return err
				})
		}(index, unit)
	}

	close(start)
	waitGroup.Wait()

	succeeded := 0
	for _, err := range results {
		if err == nil {
			succeeded++
			continue
		}
		if !errors.Is(err, domainstorage.ErrStorageAllocationExceedsQuantity) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 {
		t.Fatalf("succeeded = %d, want 1", succeeded)
	}

	// 合計が所有数量を超えていない。
	_, total, err := fixture.allocations.SumQuantityByItemIDForUpdate(
		t.Context(), fixture.ownerID, shirt.ID, 0)
	if err != nil {
		t.Fatalf("SumQuantityByItemIDForUpdate: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
}

// TestStorageAllocationRepository_transaction_rollback は、
// transaction内の後続処理が失敗した場合に割当が残らないことを確認する。
func TestStorageAllocationRepository_transaction_rollback(t *testing.T) {
	fixture := newStorageRepositoryFixture(t)

	bag := fixture.newUnit(t, fixture.ownerID, "衣服圧縮バッグ", nil, 10)
	shirt := fixture.insertRawItem(t, "半袖シャツ", 5, nil)

	transactionManager := infrapostgresql.NewTransactionManager(testPool)
	sentinel := errors.New("後続処理の失敗")

	err := transactionManager.WithinTransaction(
		context.Background(),
		func(ctx context.Context) error {
			allocation, err := domainstorage.NewStorageAllocation(
				uuid.New(), fixture.ownerID, bag, shirt, 1, time.Now().UTC())
			if err != nil {
				return err
			}
			if _, err := fixture.allocations.Create(ctx, allocation); err != nil {
				return err
			}
			return sentinel
		})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want sentinel", err)
	}

	count, err := fixture.allocations.CountByStorageUnitID(
		t.Context(), fixture.ownerID, bag.ID())
	if err != nil {
		t.Fatalf("CountByStorageUnitID: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (rollbackされている)", count)
	}
}

func mustCreateAllocation(
	t *testing.T,
	fixture storageRepositoryFixture,
	unit domainstorage.StorageUnit,
	allocatedItem domainstorage.AllocatedItem,
	quantity int32,
) domainstorage.StorageAllocation {
	t.Helper()

	allocation, err := domainstorage.NewStorageAllocation(
		uuid.New(), fixture.ownerID, unit, allocatedItem, quantity, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewStorageAllocation: %v", err)
	}
	created, err := fixture.allocations.Create(t.Context(), allocation)
	if err != nil {
		t.Fatalf("Create allocation: %v", err)
	}
	return created
}
