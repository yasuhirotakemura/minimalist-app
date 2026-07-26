package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	applicationaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/audit"
	applicationstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/storage"
	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

var (
	testNow    = time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	ownerID    = domainauth.UserID(1)
	intruderID = domainauth.UserID(2)
)

func pointerTo[T any](value T) *T { return &value }

// fixture は収納ユースケースのtest環境。
type fixture struct {
	units       *fakeStorageUnitRepository
	allocations *fakeStorageAllocationRepository
	auditLogs   *fakeAuditLogRepository

	createUnit       *applicationstorage.CreateStorageUnitService
	updateUnit       *applicationstorage.UpdateStorageUnitService
	archiveUnit      *applicationstorage.ArchiveStorageUnitService
	restoreUnit      *applicationstorage.RestoreStorageUnitService
	getUnit          *applicationstorage.GetStorageUnitService
	listUnits        *applicationstorage.ListStorageUnitsService
	getContents      *applicationstorage.GetStorageUnitContentsService
	capacity         *applicationstorage.CalculateStorageUnitCapacityService
	assignItem       *applicationstorage.AssignItemToStorageUnitService
	updateAllocation *applicationstorage.UpdateStorageAllocationService
	removeAllocation *applicationstorage.RemoveStorageAllocationService
	replace          *applicationstorage.ReplaceStorageAllocationsService
	listItemStorage  *applicationstorage.ListItemStorageAllocationsService
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	units := newFakeStorageUnitRepository()
	allocations := newFakeStorageAllocationRepository()
	auditLogs := newFakeAuditLogRepository()
	systemClock := &fixedClock{now: testNow}
	publicIDGenerator := &sequentialPublicIDGenerator{}
	transactionManager := transaction.NewPassthroughManager()

	dependencies := applicationstorage.Dependencies{
		StorageUnits: units,
		Allocations:  allocations,
		AuditRecorder: applicationaudit.NewRecorder(
			auditLogs, publicIDGenerator, systemClock),
	}

	return &fixture{
		units:       units,
		allocations: allocations,
		auditLogs:   auditLogs,
		createUnit: applicationstorage.NewCreateStorageUnitService(
			dependencies, publicIDGenerator, systemClock, transactionManager),
		updateUnit: applicationstorage.NewUpdateStorageUnitService(
			dependencies, systemClock, transactionManager),
		archiveUnit: applicationstorage.NewArchiveStorageUnitService(
			dependencies, systemClock, transactionManager),
		restoreUnit: applicationstorage.NewRestoreStorageUnitService(
			dependencies, systemClock, transactionManager),
		getUnit:     applicationstorage.NewGetStorageUnitService(dependencies),
		listUnits:   applicationstorage.NewListStorageUnitsService(dependencies),
		getContents: applicationstorage.NewGetStorageUnitContentsService(dependencies),
		capacity: applicationstorage.NewCalculateStorageUnitCapacityService(
			dependencies),
		assignItem: applicationstorage.NewAssignItemToStorageUnitService(
			dependencies, publicIDGenerator, systemClock, transactionManager),
		updateAllocation: applicationstorage.NewUpdateStorageAllocationService(
			dependencies, systemClock, transactionManager),
		removeAllocation: applicationstorage.NewRemoveStorageAllocationService(
			dependencies, systemClock, transactionManager),
		replace: applicationstorage.NewReplaceStorageAllocationsService(
			dependencies, publicIDGenerator, systemClock, transactionManager),
		listItemStorage: applicationstorage.NewListItemStorageAllocationsService(dependencies),
	}
}

// createUnitFor は収納単位を1件作成する。
func (f *fixture) createUnitFor(
	t *testing.T,
	userID domainauth.UserID,
	name string,
	parentPublicID *uuid.UUID,
) applicationstorage.StorageUnitResult {
	t.Helper()

	result, err := f.createUnit.Execute(context.Background(),
		applicationstorage.CreateStorageUnitParams{
			UserID: userID,
			Attributes: applicationstorage.AttributesParams{
				Name:                      name,
				StorageTypeCode:           "bag",
				MobilityClassCode:         "daily_bag",
				ParentStorageUnitPublicID: parentPublicID,
				TareWeightGram:            pointerTo(int32(500)),
			},
		})
	if err != nil {
		t.Fatalf("CreateStorageUnit(%s) returned error: %v", name, err)
	}
	return result.StorageUnit
}

// addItem は割当対象のアイテムを用意する。
func (f *fixture) addItem(
	itemID int64,
	quantity int32,
	weightGram *int32,
) domainstorage.AllocatedItem {
	allocatedItem := domainstorage.AllocatedItem{
		ID:         domainitem.ItemID(itemID),
		PublicID:   uuid.MustParse("018f8d0a-0000-7a3d-9e4f-" + padHex(int(itemID), 12)),
		Name:       "アイテム",
		UnitName:   "個",
		Quantity:   quantity,
		WeightGram: weightGram,
	}
	f.allocations.addItem(allocatedItem)
	return allocatedItem
}

func TestCreateStorageUnitService(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "日常リュック", nil)

	if unit.Name != "日常リュック" {
		t.Errorf("Name = %q, want 日常リュック", unit.Name)
	}
	if unit.Depth != 1 {
		t.Errorf("Depth = %d, want 1", unit.Depth)
	}
	if unit.Version != 1 {
		t.Errorf("Version = %d, want 1", unit.Version)
	}
	if !fixture.auditLogs.hasAction(domainaudit.ActionStorageUnitCreated) {
		t.Errorf("audit actions = %v, want storage_unit_created", fixture.auditLogs.actions())
	}
}

func TestCreateStorageUnitServiceRejectsParentOfOtherUser(t *testing.T) {
	fixture := newFixture(t)

	intruderUnit := fixture.createUnitFor(t, intruderID, "他人のリュック", nil)

	// 他ユーザーの収納単位を親に指定しても、存在有無を公開せず404相当を返す。
	_, err := fixture.createUnit.Execute(context.Background(),
		applicationstorage.CreateStorageUnitParams{
			UserID: ownerID,
			Attributes: applicationstorage.AttributesParams{
				Name:                      "ポーチ",
				StorageTypeCode:           "pouch",
				MobilityClassCode:         "daily_bag",
				ParentStorageUnitPublicID: &intruderUnit.PublicID,
			},
		})
	if !errors.Is(err, domainstorage.ErrStorageUnitNotFound) {
		t.Fatalf("error = %v, want ErrStorageUnitNotFound", err)
	}
}

func TestCreateStorageUnitServiceRejectsFourthLevel(t *testing.T) {
	fixture := newFixture(t)

	root := fixture.createUnitFor(t, ownerID, "部屋", nil)
	middle := fixture.createUnitFor(t, ownerID, "日常リュック", &root.PublicID)
	leaf := fixture.createUnitFor(t, ownerID, "ガジェットポーチ", &middle.PublicID)

	_, err := fixture.createUnit.Execute(context.Background(),
		applicationstorage.CreateStorageUnitParams{
			UserID: ownerID,
			Attributes: applicationstorage.AttributesParams{
				Name:                      "4段目",
				StorageTypeCode:           "pouch",
				MobilityClassCode:         "daily_bag",
				ParentStorageUnitPublicID: &leaf.PublicID,
			},
		})
	if !errors.Is(err, domainstorage.ErrStorageHierarchyTooDeep) {
		t.Fatalf("error = %v, want ErrStorageHierarchyTooDeep", err)
	}
}

func TestUpdateStorageUnitServiceRejectsMoveThatExceedsDepth(t *testing.T) {
	fixture := newFixture(t)

	root := fixture.createUnitFor(t, ownerID, "部屋", nil)
	middle := fixture.createUnitFor(t, ownerID, "日常リュック", &root.PublicID)

	// 子を1つ持つ収納単位 (部分木の高さ2) を2段目へ移すと4段目が生まれる。
	movable := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	fixture.createUnitFor(t, ownerID, "圧縮袋", &movable.PublicID)

	_, err := fixture.updateUnit.Execute(context.Background(),
		applicationstorage.UpdateStorageUnitParams{
			UserID:   ownerID,
			PublicID: movable.PublicID,
			Attributes: applicationstorage.AttributesParams{
				Name:                      "衣服圧縮バッグ",
				StorageTypeCode:           "bag",
				MobilityClassCode:         "self_carry",
				ParentStorageUnitPublicID: &middle.PublicID,
			},
			ExpectedVersion: movable.Version,
		})
	if !errors.Is(err, domainstorage.ErrStorageHierarchyTooDeep) {
		t.Fatalf("error = %v, want ErrStorageHierarchyTooDeep", err)
	}
}

func TestUpdateStorageUnitServiceRejectsCircularParent(t *testing.T) {
	fixture := newFixture(t)

	root := fixture.createUnitFor(t, ownerID, "日常リュック", nil)
	child := fixture.createUnitFor(t, ownerID, "ガジェットポーチ", &root.PublicID)

	_, err := fixture.updateUnit.Execute(context.Background(),
		applicationstorage.UpdateStorageUnitParams{
			UserID:   ownerID,
			PublicID: root.PublicID,
			Attributes: applicationstorage.AttributesParams{
				Name:                      "日常リュック",
				StorageTypeCode:           "bag",
				MobilityClassCode:         "daily_bag",
				ParentStorageUnitPublicID: &child.PublicID,
			},
			ExpectedVersion: root.Version,
		})
	if !errors.Is(err, domainstorage.ErrStorageUnitCircularParent) {
		t.Fatalf("error = %v, want ErrStorageUnitCircularParent", err)
	}
}

func TestUpdateStorageUnitServiceRecordsVersionConflict(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "日常リュック", nil)

	_, err := fixture.updateUnit.Execute(context.Background(),
		applicationstorage.UpdateStorageUnitParams{
			UserID:   ownerID,
			PublicID: unit.PublicID,
			Attributes: applicationstorage.AttributesParams{
				Name:              "改名",
				StorageTypeCode:   "bag",
				MobilityClassCode: "daily_bag",
			},
			ExpectedVersion: unit.Version + 5,
		})
	if !errors.Is(err, domainstorage.ErrStorageUnitVersionConflict) {
		t.Fatalf("error = %v, want ErrStorageUnitVersionConflict", err)
	}
	// 競合は業務transactionがrollbackするため、別transactionで記録する。
	if !fixture.auditLogs.hasAction(domainaudit.ActionVersionConflictDetected) {
		t.Errorf("audit actions = %v, want version_conflict_detected",
			fixture.auditLogs.actions())
	}
}

func TestArchiveStorageUnitServiceRejectsUnitWithChildren(t *testing.T) {
	fixture := newFixture(t)

	root := fixture.createUnitFor(t, ownerID, "日常リュック", nil)
	fixture.createUnitFor(t, ownerID, "ガジェットポーチ", &root.PublicID)

	_, err := fixture.archiveUnit.Execute(context.Background(),
		applicationstorage.ArchiveStorageUnitParams{
			UserID:          ownerID,
			PublicID:        root.PublicID,
			ExpectedVersion: root.Version,
		})
	if !errors.Is(err, domainstorage.ErrStorageUnitHasChildren) {
		t.Fatalf("error = %v, want ErrStorageUnitHasChildren", err)
	}
}

func TestArchiveStorageUnitServiceRejectsUnitWithAllocations(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	allocatedItem := fixture.addItem(10, 3, pointerTo(int32(150)))

	contents := mustAssign(t, fixture, unit, allocatedItem, 2)

	_, err := fixture.archiveUnit.Execute(context.Background(),
		applicationstorage.ArchiveStorageUnitParams{
			UserID:          ownerID,
			PublicID:        unit.PublicID,
			ExpectedVersion: contents.StorageUnit.Version,
		})
	if !errors.Is(err, domainstorage.ErrStorageUnitHasAllocations) {
		t.Fatalf("error = %v, want ErrStorageUnitHasAllocations", err)
	}
}

func TestArchiveAndRestoreStorageUnitService(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "使わない箱", nil)

	archived, err := fixture.archiveUnit.Execute(context.Background(),
		applicationstorage.ArchiveStorageUnitParams{
			UserID:          ownerID,
			PublicID:        unit.PublicID,
			ExpectedVersion: unit.Version,
		})
	if err != nil {
		t.Fatalf("ArchiveStorageUnit returned error: %v", err)
	}
	if !archived.StorageUnit.IsArchived {
		t.Error("IsArchived = false, want true")
	}

	restored, err := fixture.restoreUnit.Execute(context.Background(),
		applicationstorage.RestoreStorageUnitParams{
			UserID:          ownerID,
			PublicID:        unit.PublicID,
			ExpectedVersion: archived.StorageUnit.Version,
		})
	if err != nil {
		t.Fatalf("RestoreStorageUnit returned error: %v", err)
	}
	if restored.StorageUnit.IsArchived {
		t.Error("IsArchived = true, want false")
	}

	if !fixture.auditLogs.hasAction(domainaudit.ActionStorageUnitArchived) ||
		!fixture.auditLogs.hasAction(domainaudit.ActionStorageUnitRestored) {
		t.Errorf("audit actions = %v, want archive and restore", fixture.auditLogs.actions())
	}
}

func TestGetStorageUnitServiceRejectsOtherUser(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "日常リュック", nil)

	_, err := fixture.getUnit.Execute(context.Background(),
		applicationstorage.GetStorageUnitParams{
			UserID:   intruderID,
			PublicID: unit.PublicID,
		})
	if !errors.Is(err, domainstorage.ErrStorageUnitNotFound) {
		t.Fatalf("error = %v, want ErrStorageUnitNotFound", err)
	}
}

func TestAssignItemToStorageUnitService(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	allocatedItem := fixture.addItem(10, 3, pointerTo(int32(150)))

	contents := mustAssign(t, fixture, unit, allocatedItem, 2)

	if len(contents.Allocations) != 1 {
		t.Fatalf("Allocations = %d, want 1", len(contents.Allocations))
	}
	if contents.Allocations[0].Quantity != 2 {
		t.Errorf("Quantity = %d, want 2", contents.Allocations[0].Quantity)
	}
	if contents.Allocations[0].Item.UnassignedQuantity != 1 {
		t.Errorf("UnassignedQuantity = %d, want 1",
			contents.Allocations[0].Item.UnassignedQuantity)
	}
	// 割当集合の競合を検知するため、収納単位のversionを増加させる。
	if contents.StorageUnit.Version != unit.Version+1 {
		t.Errorf("StorageUnit.Version = %d, want %d",
			contents.StorageUnit.Version, unit.Version+1)
	}
	// 500 (自重) + 150*2
	if contents.StorageUnit.Capacity.TotalWeightGram != 800 {
		t.Errorf("TotalWeightGram = %d, want 800",
			contents.StorageUnit.Capacity.TotalWeightGram)
	}
	if !fixture.auditLogs.hasAction(domainaudit.ActionStorageAllocationCreated) {
		t.Errorf("audit actions = %v, want storage_allocation_created",
			fixture.auditLogs.actions())
	}
}

func TestAssignItemToStorageUnitServiceRejectsQuantityAboveOwned(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	allocatedItem := fixture.addItem(10, 3, nil)

	_, err := fixture.assignItem.Execute(context.Background(),
		applicationstorage.AssignItemToStorageUnitParams{
			UserID:                     ownerID,
			StorageUnitPublicID:        unit.PublicID,
			ItemPublicID:               allocatedItem.PublicID,
			Quantity:                   4,
			ExpectedStorageUnitVersion: unit.Version,
		})
	if !errors.Is(err, domainstorage.ErrStorageAllocationExceedsQuantity) {
		t.Fatalf("error = %v, want ErrStorageAllocationExceedsQuantity", err)
	}
}

func TestAssignItemSplitsAcrossStorageUnitsWithinOwnedQuantity(t *testing.T) {
	fixture := newFixture(t)

	bag := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	backpack := fixture.createUnitFor(t, ownerID, "日常リュック", nil)
	allocatedItem := fixture.addItem(10, 3, pointerTo(int32(150)))

	mustAssign(t, fixture, bag, allocatedItem, 2)
	contents := mustAssign(t, fixture, backpack, allocatedItem, 1)

	if contents.Allocations[0].Item.AssignedQuantity != 3 {
		t.Errorf("AssignedQuantity = %d, want 3",
			contents.Allocations[0].Item.AssignedQuantity)
	}
	if contents.Allocations[0].Item.UnassignedQuantity != 0 {
		t.Errorf("UnassignedQuantity = %d, want 0",
			contents.Allocations[0].Item.UnassignedQuantity)
	}

	// 合計が所有数量に達したため、これ以上は割り当てられない。
	third := fixture.createUnitFor(t, ownerID, "予備の箱", nil)
	_, err := fixture.assignItem.Execute(context.Background(),
		applicationstorage.AssignItemToStorageUnitParams{
			UserID:                     ownerID,
			StorageUnitPublicID:        third.PublicID,
			ItemPublicID:               allocatedItem.PublicID,
			Quantity:                   1,
			ExpectedStorageUnitVersion: third.Version,
		})
	if !errors.Is(err, domainstorage.ErrStorageAllocationExceedsQuantity) {
		t.Fatalf("error = %v, want ErrStorageAllocationExceedsQuantity", err)
	}
}

func TestAssignItemToStorageUnitServiceRejectsDuplicate(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	allocatedItem := fixture.addItem(10, 5, nil)

	contents := mustAssign(t, fixture, unit, allocatedItem, 1)

	_, err := fixture.assignItem.Execute(context.Background(),
		applicationstorage.AssignItemToStorageUnitParams{
			UserID:                     ownerID,
			StorageUnitPublicID:        unit.PublicID,
			ItemPublicID:               allocatedItem.PublicID,
			Quantity:                   1,
			ExpectedStorageUnitVersion: contents.StorageUnit.Version,
		})
	if !errors.Is(err, domainstorage.ErrStorageAllocationAlreadyExists) {
		t.Fatalf("error = %v, want ErrStorageAllocationAlreadyExists", err)
	}
}

func TestAssignItemToStorageUnitServiceDetectsStorageUnitVersionConflict(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	allocatedItem := fixture.addItem(10, 5, nil)

	_, err := fixture.assignItem.Execute(context.Background(),
		applicationstorage.AssignItemToStorageUnitParams{
			UserID:                     ownerID,
			StorageUnitPublicID:        unit.PublicID,
			ItemPublicID:               allocatedItem.PublicID,
			Quantity:                   1,
			ExpectedStorageUnitVersion: unit.Version + 3,
		})
	if !errors.Is(err, domainstorage.ErrStorageUnitVersionConflict) {
		t.Fatalf("error = %v, want ErrStorageUnitVersionConflict", err)
	}
}

func TestAssignItemToStorageUnitServiceRollsBackOnFailure(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	allocatedItem := fixture.addItem(10, 5, nil)

	// 割当のinsertが失敗した場合、収納単位のversionも増加させない。
	fixture.allocations.failCreate = true
	_, err := fixture.assignItem.Execute(context.Background(),
		applicationstorage.AssignItemToStorageUnitParams{
			UserID:                     ownerID,
			StorageUnitPublicID:        unit.PublicID,
			ItemPublicID:               allocatedItem.PublicID,
			Quantity:                   1,
			ExpectedStorageUnitVersion: unit.Version,
		})
	if !errors.Is(err, errRepository) {
		t.Fatalf("error = %v, want errRepository", err)
	}

	current, err := fixture.getUnit.Execute(context.Background(),
		applicationstorage.GetStorageUnitParams{UserID: ownerID, PublicID: unit.PublicID})
	if err != nil {
		t.Fatalf("GetStorageUnit returned error: %v", err)
	}
	if current.StorageUnit.Version != unit.Version {
		t.Errorf("Version = %d, want %d (unchanged)",
			current.StorageUnit.Version, unit.Version)
	}
}

func TestUpdateStorageAllocationService(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	allocatedItem := fixture.addItem(10, 5, pointerTo(int32(100)))
	contents := mustAssign(t, fixture, unit, allocatedItem, 2)
	allocation := contents.Allocations[0]

	updated, err := fixture.updateAllocation.Execute(context.Background(),
		applicationstorage.UpdateStorageAllocationParams{
			UserID:                     ownerID,
			StorageUnitPublicID:        unit.PublicID,
			AllocationPublicID:         allocation.PublicID,
			Quantity:                   4,
			ExpectedVersion:            allocation.Version,
			ExpectedStorageUnitVersion: contents.StorageUnit.Version,
		})
	if err != nil {
		t.Fatalf("UpdateStorageAllocation returned error: %v", err)
	}
	if updated.Allocations[0].Quantity != 4 {
		t.Errorf("Quantity = %d, want 4", updated.Allocations[0].Quantity)
	}
	if updated.Allocations[0].Version != allocation.Version+1 {
		t.Errorf("Version = %d, want %d",
			updated.Allocations[0].Version, allocation.Version+1)
	}

	// 変更後の値が所有数量を超える場合は拒否する。
	_, err = fixture.updateAllocation.Execute(context.Background(),
		applicationstorage.UpdateStorageAllocationParams{
			UserID:                     ownerID,
			StorageUnitPublicID:        unit.PublicID,
			AllocationPublicID:         allocation.PublicID,
			Quantity:                   6,
			ExpectedVersion:            updated.Allocations[0].Version,
			ExpectedStorageUnitVersion: updated.StorageUnit.Version,
		})
	if !errors.Is(err, domainstorage.ErrStorageAllocationExceedsQuantity) {
		t.Fatalf("error = %v, want ErrStorageAllocationExceedsQuantity", err)
	}
}

func TestUpdateStorageAllocationServiceDetectsAllocationVersionConflict(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	allocatedItem := fixture.addItem(10, 5, nil)
	contents := mustAssign(t, fixture, unit, allocatedItem, 2)
	allocation := contents.Allocations[0]

	_, err := fixture.updateAllocation.Execute(context.Background(),
		applicationstorage.UpdateStorageAllocationParams{
			UserID:                     ownerID,
			StorageUnitPublicID:        unit.PublicID,
			AllocationPublicID:         allocation.PublicID,
			Quantity:                   3,
			ExpectedVersion:            allocation.Version + 9,
			ExpectedStorageUnitVersion: contents.StorageUnit.Version,
		})
	if !errors.Is(err, domainstorage.ErrStorageAllocationVersionConflict) {
		t.Fatalf("error = %v, want ErrStorageAllocationVersionConflict", err)
	}
}

func TestRemoveStorageAllocationService(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	allocatedItem := fixture.addItem(10, 5, nil)
	contents := mustAssign(t, fixture, unit, allocatedItem, 2)
	allocation := contents.Allocations[0]

	removed, err := fixture.removeAllocation.Execute(context.Background(),
		applicationstorage.RemoveStorageAllocationParams{
			UserID:                     ownerID,
			StorageUnitPublicID:        unit.PublicID,
			AllocationPublicID:         allocation.PublicID,
			ExpectedVersion:            allocation.Version,
			ExpectedStorageUnitVersion: contents.StorageUnit.Version,
		})
	if err != nil {
		t.Fatalf("RemoveStorageAllocation returned error: %v", err)
	}
	if len(removed.Allocations) != 0 {
		t.Errorf("Allocations = %d, want 0", len(removed.Allocations))
	}
	if !fixture.auditLogs.hasAction(domainaudit.ActionStorageAllocationDeleted) {
		t.Errorf("audit actions = %v, want storage_allocation_deleted",
			fixture.auditLogs.actions())
	}
}

func TestReplaceStorageAllocationsService(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	shirt := fixture.addItem(10, 5, pointerTo(int32(150)))
	towel := fixture.addItem(11, 2, pointerTo(int32(300)))

	contents := mustAssign(t, fixture, unit, shirt, 2)

	replaced, err := fixture.replace.Execute(context.Background(),
		applicationstorage.ReplaceStorageAllocationsParams{
			UserID:              ownerID,
			StorageUnitPublicID: unit.PublicID,
			Allocations: []applicationstorage.AllocationInput{
				{ItemPublicID: shirt.PublicID, Quantity: 3},
				{ItemPublicID: towel.PublicID, Quantity: 2},
			},
			ExpectedStorageUnitVersion: contents.StorageUnit.Version,
		})
	if err != nil {
		t.Fatalf("ReplaceStorageAllocations returned error: %v", err)
	}
	if len(replaced.Allocations) != 2 {
		t.Fatalf("Allocations = %d, want 2", len(replaced.Allocations))
	}
	// 500 (自重) + 150*3 + 300*2
	if replaced.StorageUnit.Capacity.TotalWeightGram != 1550 {
		t.Errorf("TotalWeightGram = %d, want 1550",
			replaced.StorageUnit.Capacity.TotalWeightGram)
	}
	if !fixture.auditLogs.hasAction(domainaudit.ActionStorageAllocationsReplaced) {
		t.Errorf("audit actions = %v, want storage_allocations_replaced",
			fixture.auditLogs.actions())
	}
}

func TestReplaceStorageAllocationsServiceEmptiesContents(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	shirt := fixture.addItem(10, 5, nil)
	contents := mustAssign(t, fixture, unit, shirt, 2)

	replaced, err := fixture.replace.Execute(context.Background(),
		applicationstorage.ReplaceStorageAllocationsParams{
			UserID:                     ownerID,
			StorageUnitPublicID:        unit.PublicID,
			Allocations:                []applicationstorage.AllocationInput{},
			ExpectedStorageUnitVersion: contents.StorageUnit.Version,
		})
	if err != nil {
		t.Fatalf("ReplaceStorageAllocations returned error: %v", err)
	}
	if len(replaced.Allocations) != 0 {
		t.Errorf("Allocations = %d, want 0", len(replaced.Allocations))
	}
}

func TestReplaceStorageAllocationsServiceRejectsDuplicatedItem(t *testing.T) {
	fixture := newFixture(t)

	unit := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	shirt := fixture.addItem(10, 5, nil)

	_, err := fixture.replace.Execute(context.Background(),
		applicationstorage.ReplaceStorageAllocationsParams{
			UserID:              ownerID,
			StorageUnitPublicID: unit.PublicID,
			Allocations: []applicationstorage.AllocationInput{
				{ItemPublicID: shirt.PublicID, Quantity: 1},
				{ItemPublicID: shirt.PublicID, Quantity: 2},
			},
			ExpectedStorageUnitVersion: unit.Version,
		})
	if !errors.Is(err, domainstorage.ErrStorageAllocationDuplicatedItem) {
		t.Fatalf("error = %v, want ErrStorageAllocationDuplicatedItem", err)
	}
}

func TestReplaceStorageAllocationsServiceRollsBackOnQuantityViolation(t *testing.T) {
	fixture := newFixture(t)

	bag := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	backpack := fixture.createUnitFor(t, ownerID, "日常リュック", nil)
	shirt := fixture.addItem(10, 3, nil)

	// 別の収納単位へ2枚割り当て済み。残り1枚しか割り当てられない。
	mustAssign(t, fixture, backpack, shirt, 2)
	contents := mustAssign(t, fixture, bag, shirt, 1)

	_, err := fixture.replace.Execute(context.Background(),
		applicationstorage.ReplaceStorageAllocationsParams{
			UserID:              ownerID,
			StorageUnitPublicID: bag.PublicID,
			Allocations: []applicationstorage.AllocationInput{
				{ItemPublicID: shirt.PublicID, Quantity: 2},
			},
			ExpectedStorageUnitVersion: contents.StorageUnit.Version,
		})
	if !errors.Is(err, domainstorage.ErrStorageAllocationExceedsQuantity) {
		t.Fatalf("error = %v, want ErrStorageAllocationExceedsQuantity", err)
	}

	// 既存の割当が消えていないことを確認する。
	current, err := fixture.getContents.Execute(context.Background(),
		applicationstorage.GetStorageUnitContentsParams{
			UserID: ownerID, PublicID: bag.PublicID,
		})
	if err != nil {
		t.Fatalf("GetStorageUnitContents returned error: %v", err)
	}
	if len(current.Allocations) != 1 || current.Allocations[0].Quantity != 1 {
		t.Errorf("allocations = %+v, want 1 allocation of quantity 1", current.Allocations)
	}
}

func TestListItemStorageAllocationsService(t *testing.T) {
	fixture := newFixture(t)

	bag := fixture.createUnitFor(t, ownerID, "衣服圧縮バッグ", nil)
	backpack := fixture.createUnitFor(t, ownerID, "日常リュック", nil)
	shirt := fixture.addItem(10, 3, nil)

	mustAssign(t, fixture, bag, shirt, 2)
	mustAssign(t, fixture, backpack, shirt, 1)

	result, err := fixture.listItemStorage.Execute(context.Background(),
		applicationstorage.ListItemStorageAllocationsParams{
			UserID: ownerID, ItemPublicID: shirt.PublicID,
		})
	if err != nil {
		t.Fatalf("ListItemStorageAllocations returned error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("Items = %d, want 2", len(result.Items))
	}
	if result.AssignedQuantity != 3 || result.UnassignedQuantity != 0 {
		t.Errorf("assigned/unassigned = %d/%d, want 3/0",
			result.AssignedQuantity, result.UnassignedQuantity)
	}
}

func TestCalculateStorageUnitCapacityServiceAggregatesDescendants(t *testing.T) {
	fixture := newFixture(t)

	backpack := fixture.createUnitFor(t, ownerID, "日常リュック", nil)
	pouch := fixture.createUnitFor(t, ownerID, "ガジェットポーチ", &backpack.PublicID)

	charger := fixture.addItem(20, 1, pointerTo(int32(200)))
	mustAssign(t, fixture, pouch, charger, 1)

	result, err := fixture.capacity.Execute(context.Background(),
		applicationstorage.CalculateStorageUnitCapacityParams{
			UserID: ownerID, PublicID: backpack.PublicID,
		})
	if err != nil {
		t.Fatalf("CalculateStorageUnitCapacity returned error: %v", err)
	}

	// 親の自重500 + 子孫 (子の自重500 + 中身200)
	if result.Capacity.DescendantWeightGram != 700 {
		t.Errorf("DescendantWeightGram = %d, want 700", result.Capacity.DescendantWeightGram)
	}
	if result.Capacity.TotalWeightGram != 1200 {
		t.Errorf("TotalWeightGram = %d, want 1200", result.Capacity.TotalWeightGram)
	}
	// アイテムの容積が未設定のため不完全な集計であることを示す。
	if !result.Capacity.HasUnknownVolume {
		t.Error("HasUnknownVolume = false, want true")
	}
}

func TestListStorageUnitsServiceExcludesOtherUsers(t *testing.T) {
	fixture := newFixture(t)

	fixture.createUnitFor(t, ownerID, "日常リュック", nil)
	fixture.createUnitFor(t, intruderID, "他人のリュック", nil)

	result, err := fixture.listUnits.Execute(context.Background(),
		applicationstorage.ListStorageUnitsParams{UserID: ownerID})
	if err != nil {
		t.Fatalf("ListStorageUnits returned error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("Items = %d, want 1", len(result.Items))
	}
	if result.Items[0].Name != "日常リュック" {
		t.Errorf("Name = %q, want 日常リュック", result.Items[0].Name)
	}
	if result.Pagination.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", result.Pagination.TotalCount)
	}
}

// mustAssign は割当を1件作成し、更新後の内容を返す。
func mustAssign(
	t *testing.T,
	fixture *fixture,
	unit applicationstorage.StorageUnitResult,
	allocatedItem domainstorage.AllocatedItem,
	quantity int32,
) applicationstorage.StorageUnitContentsResult {
	t.Helper()

	current, err := fixture.getUnit.Execute(context.Background(),
		applicationstorage.GetStorageUnitParams{UserID: ownerID, PublicID: unit.PublicID})
	if err != nil {
		t.Fatalf("GetStorageUnit returned error: %v", err)
	}

	contents, err := fixture.assignItem.Execute(context.Background(),
		applicationstorage.AssignItemToStorageUnitParams{
			UserID:                     ownerID,
			StorageUnitPublicID:        unit.PublicID,
			ItemPublicID:               allocatedItem.PublicID,
			Quantity:                   quantity,
			ExpectedStorageUnitVersion: current.StorageUnit.Version,
		})
	if err != nil {
		t.Fatalf("AssignItemToStorageUnit returned error: %v", err)
	}
	return contents
}
