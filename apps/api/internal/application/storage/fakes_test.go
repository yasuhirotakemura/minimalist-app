package storage_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
)

// errRepository はrepositoryの技術的失敗を模したerror。
var errRepository = errors.New("repository failure")

// fixedClock は固定時刻を返す。
type fixedClock struct {
	now time.Time
}

func (c *fixedClock) Now() time.Time { return c.now }

// sequentialPublicIDGenerator は決定的なUUIDを返す。
type sequentialPublicIDGenerator struct {
	mutex   sync.Mutex
	counter int
}

func (g *sequentialPublicIDGenerator) NewPublicID() (uuid.UUID, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.counter++
	return uuid.MustParse(
		"018f8d0a-1c2b-7a3d-9e4f-" + padHex(g.counter, 12)), nil
}

func padHex(value, width int) string {
	digits := "0123456789abcdef"
	buffer := make([]byte, width)
	for index := width - 1; index >= 0; index-- {
		buffer[index] = digits[value%16]
		value /= 16
	}
	return string(buffer)
}

// fakeStorageUnitRepository は収納単位repositoryのin-memory実装。
type fakeStorageUnitRepository struct {
	mutex      sync.Mutex
	nextID     int64
	units      map[domainstorage.StorageUnitID]domainstorage.StorageUnit
	failCreate bool
}

func newFakeStorageUnitRepository() *fakeStorageUnitRepository {
	return &fakeStorageUnitRepository{
		nextID: 1,
		units:  map[domainstorage.StorageUnitID]domainstorage.StorageUnit{},
	}
}

var _ domainstorage.StorageUnitRepository = (*fakeStorageUnitRepository)(nil)

func (r *fakeStorageUnitRepository) Create(
	_ context.Context, unit domainstorage.StorageUnit,
) (domainstorage.StorageUnit, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.failCreate {
		return domainstorage.StorageUnit{}, errRepository
	}

	id := domainstorage.StorageUnitID(r.nextID)
	r.nextID++
	stored := unit.WithID(id)
	r.units[id] = stored
	return stored, nil
}

func (r *fakeStorageUnitRepository) FindByPublicID(
	_ context.Context, userID domainauth.UserID, publicID uuid.UUID,
) (domainstorage.StorageUnit, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.findLocked(userID, publicID)
}

// findLocked はmutex保持中の検索。他ユーザーのpublicIdは存在しない扱いとする。
func (r *fakeStorageUnitRepository) findLocked(
	userID domainauth.UserID, publicID uuid.UUID,
) (domainstorage.StorageUnit, error) {
	for _, unit := range r.units {
		if unit.PublicID() == publicID && unit.UserID() == userID {
			return r.withChildCountLocked(unit), nil
		}
	}
	return domainstorage.StorageUnit{}, domainstorage.ErrStorageUnitNotFound
}

// withChildCountLocked はarchive前の子件数を数えた複製を返す。
func (r *fakeStorageUnitRepository) withChildCountLocked(
	unit domainstorage.StorageUnit,
) domainstorage.StorageUnit {
	var childCount int32
	for _, candidate := range r.units {
		if candidate.IsArchived() || !candidate.HasParent() {
			continue
		}
		if candidate.Parent().ID == unit.ID() {
			childCount++
		}
	}

	return domainstorage.ReconstructStorageUnit(domainstorage.ReconstructStorageUnitParams{
		ID:         unit.ID(),
		PublicID:   unit.PublicID(),
		UserID:     unit.UserID(),
		Attributes: unit.Attributes(),
		Ancestors:  unit.Ancestors(),
		ChildCount: childCount,
		CreatedAt:  unit.CreatedAt(),
		UpdatedAt:  unit.UpdatedAt(),
		ArchivedAt: unit.ArchivedAt(),
		Version:    unit.Version(),
	})
}

func (r *fakeStorageUnitRepository) Update(
	_ context.Context, unit domainstorage.StorageUnit, expectedVersion int32,
) (domainstorage.StorageUnit, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	stored, ok := r.units[unit.ID()]
	if !ok || stored.UserID() != unit.UserID() {
		return domainstorage.StorageUnit{}, domainstorage.ErrStorageUnitNotFound
	}
	if stored.Version() != expectedVersion {
		return domainstorage.StorageUnit{}, domainstorage.ErrStorageUnitVersionConflict
	}

	r.units[unit.ID()] = unit
	return r.withChildCountLocked(unit), nil
}

func (r *fakeStorageUnitRepository) Archive(
	_ context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
	expectedVersion int32,
	archivedAt time.Time,
) (domainstorage.StorageUnit, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	stored, err := r.findLocked(userID, publicID)
	if err != nil {
		return domainstorage.StorageUnit{}, err
	}
	if stored.Version() != expectedVersion {
		return domainstorage.StorageUnit{}, domainstorage.ErrStorageUnitVersionConflict
	}

	archived := domainstorage.ReconstructStorageUnit(
		domainstorage.ReconstructStorageUnitParams{
			ID:         stored.ID(),
			PublicID:   stored.PublicID(),
			UserID:     stored.UserID(),
			Attributes: stored.Attributes(),
			Ancestors:  stored.Ancestors(),
			CreatedAt:  stored.CreatedAt(),
			UpdatedAt:  archivedAt,
			ArchivedAt: &archivedAt,
			Version:    expectedVersion + 1,
		})
	r.units[stored.ID()] = archived
	return r.withChildCountLocked(archived), nil
}

func (r *fakeStorageUnitRepository) Restore(
	_ context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
	expectedVersion int32,
	now time.Time,
) (domainstorage.StorageUnit, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	stored, err := r.findLocked(userID, publicID)
	if err != nil {
		return domainstorage.StorageUnit{}, err
	}
	if stored.Version() != expectedVersion {
		return domainstorage.StorageUnit{}, domainstorage.ErrStorageUnitVersionConflict
	}

	restored := domainstorage.ReconstructStorageUnit(
		domainstorage.ReconstructStorageUnitParams{
			ID:         stored.ID(),
			PublicID:   stored.PublicID(),
			UserID:     stored.UserID(),
			Attributes: stored.Attributes(),
			Ancestors:  stored.Ancestors(),
			CreatedAt:  stored.CreatedAt(),
			UpdatedAt:  now,
			Version:    expectedVersion + 1,
		})
	r.units[stored.ID()] = restored
	return r.withChildCountLocked(restored), nil
}

func (r *fakeStorageUnitRepository) TouchVersion(
	_ context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
	expectedVersion int32,
	now time.Time,
) (domainstorage.StorageUnit, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	stored, err := r.findLocked(userID, publicID)
	if err != nil {
		return domainstorage.StorageUnit{}, err
	}
	if stored.Version() != expectedVersion {
		return domainstorage.StorageUnit{}, domainstorage.ErrStorageUnitVersionConflict
	}

	touched := domainstorage.ReconstructStorageUnit(
		domainstorage.ReconstructStorageUnitParams{
			ID:         stored.ID(),
			PublicID:   stored.PublicID(),
			UserID:     stored.UserID(),
			Attributes: stored.Attributes(),
			Ancestors:  stored.Ancestors(),
			CreatedAt:  stored.CreatedAt(),
			UpdatedAt:  now,
			ArchivedAt: stored.ArchivedAt(),
			Version:    expectedVersion + 1,
		})
	r.units[stored.ID()] = touched
	return r.withChildCountLocked(touched), nil
}

func (r *fakeStorageUnitRepository) List(
	_ context.Context, userID domainauth.UserID, criteria domainstorage.ListCriteria,
) ([]domainstorage.StorageUnit, error) {
	units := r.matching(userID, criteria)
	if int(criteria.Offset) >= len(units) {
		return nil, nil
	}
	end := int(criteria.Offset + criteria.Limit)
	if end > len(units) {
		end = len(units)
	}
	return units[criteria.Offset:end], nil
}

func (r *fakeStorageUnitRepository) Count(
	_ context.Context, userID domainauth.UserID, criteria domainstorage.ListCriteria,
) (int64, error) {
	return int64(len(r.matching(userID, criteria))), nil
}

// matching は絞り込み条件に一致する収納単位をsortOrder昇順で返す。
func (r *fakeStorageUnitRepository) matching(
	userID domainauth.UserID, criteria domainstorage.ListCriteria,
) []domainstorage.StorageUnit {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	matched := make([]domainstorage.StorageUnit, 0, len(r.units))
	for _, unit := range r.units {
		if unit.UserID() != userID {
			continue
		}
		if unit.IsArchived() && !criteria.IncludeArchived {
			continue
		}
		if criteria.RootOnly && unit.HasParent() {
			continue
		}
		if criteria.StorageType != nil &&
			unit.Attributes().StorageType != *criteria.StorageType {
			continue
		}
		if criteria.MobilityClass != nil &&
			unit.Attributes().MobilityClass != *criteria.MobilityClass {
			continue
		}
		if criteria.ParentPublicID != nil &&
			(!unit.HasParent() || unit.Parent().PublicID != *criteria.ParentPublicID) {
			continue
		}
		matched = append(matched, r.withChildCountLocked(unit))
	}

	sort.Slice(matched, func(left, right int) bool {
		if matched[left].Attributes().SortOrder != matched[right].Attributes().SortOrder {
			return matched[left].Attributes().SortOrder <
				matched[right].Attributes().SortOrder
		}
		return matched[left].ID() < matched[right].ID()
	})
	return matched
}

func (r *fakeStorageUnitRepository) ListAll(
	_ context.Context, userID domainauth.UserID, includeArchived bool,
) ([]domainstorage.StorageUnit, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	units := make([]domainstorage.StorageUnit, 0, len(r.units))
	for _, unit := range r.units {
		if unit.UserID() != userID {
			continue
		}
		if unit.IsArchived() && !includeArchived {
			continue
		}
		units = append(units, r.withChildCountLocked(unit))
	}
	sort.Slice(units, func(left, right int) bool { return units[left].ID() < units[right].ID() })
	return units, nil
}

func (r *fakeStorageUnitRepository) ListChildren(
	_ context.Context, userID domainauth.UserID, parentID domainstorage.StorageUnitID,
) ([]domainstorage.StorageUnit, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	children := make([]domainstorage.StorageUnit, 0)
	for _, unit := range r.units {
		if unit.UserID() != userID || unit.IsArchived() || !unit.HasParent() {
			continue
		}
		if unit.Parent().ID == parentID {
			children = append(children, r.withChildCountLocked(unit))
		}
	}
	return children, nil
}

func (r *fakeStorageUnitRepository) CountActiveChildren(
	_ context.Context, userID domainauth.UserID, parentID domainstorage.StorageUnitID,
) (int64, error) {
	children, err := r.ListChildren(context.Background(), userID, parentID)
	if err != nil {
		return 0, err
	}
	return int64(len(children)), nil
}

// fakeStorageAllocationRepository は収納割当repositoryのin-memory実装。
type fakeStorageAllocationRepository struct {
	mutex       sync.Mutex
	nextID      int64
	allocations map[domainstorage.AllocationID]domainstorage.StorageAllocation
	// items は割当対象として解決できるアイテム。
	items map[uuid.UUID]domainstorage.AllocatedItem
	// failCreate はtransaction rollbackの検証で使用する。
	failCreate bool
}

func newFakeStorageAllocationRepository() *fakeStorageAllocationRepository {
	return &fakeStorageAllocationRepository{
		nextID:      1,
		allocations: map[domainstorage.AllocationID]domainstorage.StorageAllocation{},
		items:       map[uuid.UUID]domainstorage.AllocatedItem{},
	}
}

var _ domainstorage.StorageAllocationRepository = (*fakeStorageAllocationRepository)(nil)

// addItem はtestから割当対象アイテムを用意する。
func (r *fakeStorageAllocationRepository) addItem(allocatedItem domainstorage.AllocatedItem) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.items[allocatedItem.PublicID] = allocatedItem
}

func (r *fakeStorageAllocationRepository) Create(
	_ context.Context, allocation domainstorage.StorageAllocation,
) (domainstorage.StorageAllocation, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.failCreate {
		return domainstorage.StorageAllocation{}, errRepository
	}
	for _, stored := range r.allocations {
		if stored.StorageUnitID() == allocation.StorageUnitID() &&
			stored.ItemID() == allocation.ItemID() {
			return domainstorage.StorageAllocation{},
				domainstorage.ErrStorageAllocationAlreadyExists
		}
	}

	id := domainstorage.AllocationID(r.nextID)
	r.nextID++
	stored := allocation.WithID(id)
	r.allocations[id] = stored
	return stored, nil
}

func (r *fakeStorageAllocationRepository) FindByPublicID(
	_ context.Context, userID domainauth.UserID, publicID uuid.UUID,
) (domainstorage.StorageAllocation, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, allocation := range r.allocations {
		if allocation.PublicID() == publicID && allocation.UserID() == userID {
			return allocation, nil
		}
	}
	return domainstorage.StorageAllocation{}, domainstorage.ErrStorageAllocationNotFound
}

func (r *fakeStorageAllocationRepository) UpdateQuantity(
	_ context.Context, allocation domainstorage.StorageAllocation, expectedVersion int32,
) (domainstorage.StorageAllocation, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	stored, ok := r.allocations[allocation.ID()]
	if !ok || stored.UserID() != allocation.UserID() {
		return domainstorage.StorageAllocation{},
			domainstorage.ErrStorageAllocationNotFound
	}
	if stored.Version() != expectedVersion {
		return domainstorage.StorageAllocation{},
			domainstorage.ErrStorageAllocationVersionConflict
	}

	r.allocations[allocation.ID()] = allocation
	return allocation, nil
}

func (r *fakeStorageAllocationRepository) Delete(
	_ context.Context, userID domainauth.UserID, publicID uuid.UUID, expectedVersion int32,
) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for id, allocation := range r.allocations {
		if allocation.PublicID() != publicID || allocation.UserID() != userID {
			continue
		}
		if allocation.Version() != expectedVersion {
			return domainstorage.ErrStorageAllocationVersionConflict
		}
		delete(r.allocations, id)
		return nil
	}
	return domainstorage.ErrStorageAllocationNotFound
}

func (r *fakeStorageAllocationRepository) DeleteByStorageUnitID(
	_ context.Context, userID domainauth.UserID, storageUnitID domainstorage.StorageUnitID,
) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for id, allocation := range r.allocations {
		if allocation.UserID() == userID && allocation.StorageUnitID() == storageUnitID {
			delete(r.allocations, id)
		}
	}
	return nil
}

func (r *fakeStorageAllocationRepository) ListByStorageUnitID(
	ctx context.Context, userID domainauth.UserID, storageUnitID domainstorage.StorageUnitID,
) ([]domainstorage.StorageAllocation, error) {
	byUnitID, err := r.ListByStorageUnitIDs(
		ctx, userID, []domainstorage.StorageUnitID{storageUnitID})
	if err != nil {
		return nil, err
	}
	return byUnitID[storageUnitID], nil
}

func (r *fakeStorageAllocationRepository) ListByStorageUnitIDs(
	_ context.Context, userID domainauth.UserID, storageUnitIDs []domainstorage.StorageUnitID,
) (map[domainstorage.StorageUnitID][]domainstorage.StorageAllocation, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	wanted := make(map[domainstorage.StorageUnitID]struct{}, len(storageUnitIDs))
	for _, id := range storageUnitIDs {
		wanted[id] = struct{}{}
	}

	result := map[domainstorage.StorageUnitID][]domainstorage.StorageAllocation{}
	for _, allocation := range r.sortedLocked() {
		if allocation.UserID() != userID {
			continue
		}
		if _, ok := wanted[allocation.StorageUnitID()]; !ok {
			continue
		}
		result[allocation.StorageUnitID()] = append(
			result[allocation.StorageUnitID()], allocation)
	}
	return result, nil
}

func (r *fakeStorageAllocationRepository) ListByItemID(
	ctx context.Context, userID domainauth.UserID, itemID domainitem.ItemID,
) ([]domainstorage.StorageAllocation, error) {
	byItemID, err := r.ListByItemIDs(ctx, userID, []domainitem.ItemID{itemID})
	if err != nil {
		return nil, err
	}
	return byItemID[itemID], nil
}

func (r *fakeStorageAllocationRepository) ListByItemIDs(
	_ context.Context, userID domainauth.UserID, itemIDs []domainitem.ItemID,
) (map[domainitem.ItemID][]domainstorage.StorageAllocation, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	wanted := make(map[domainitem.ItemID]struct{}, len(itemIDs))
	for _, id := range itemIDs {
		wanted[id] = struct{}{}
	}

	result := map[domainitem.ItemID][]domainstorage.StorageAllocation{}
	for _, allocation := range r.sortedLocked() {
		if allocation.UserID() != userID {
			continue
		}
		if _, ok := wanted[allocation.ItemID()]; !ok {
			continue
		}
		result[allocation.ItemID()] = append(result[allocation.ItemID()], allocation)
	}
	return result, nil
}

// sortedLocked は決定的な順序で割当を返す。
func (r *fakeStorageAllocationRepository) sortedLocked() []domainstorage.StorageAllocation {
	allocations := make([]domainstorage.StorageAllocation, 0, len(r.allocations))
	for _, allocation := range r.allocations {
		allocations = append(allocations, allocation)
	}
	sort.Slice(allocations, func(left, right int) bool {
		return allocations[left].ID() < allocations[right].ID()
	})
	return allocations
}

func (r *fakeStorageAllocationRepository) CountByStorageUnitID(
	ctx context.Context, userID domainauth.UserID, storageUnitID domainstorage.StorageUnitID,
) (int64, error) {
	allocations, err := r.ListByStorageUnitID(ctx, userID, storageUnitID)
	if err != nil {
		return 0, err
	}
	return int64(len(allocations)), nil
}

func (r *fakeStorageAllocationRepository) SumQuantityByItemIDForUpdate(
	_ context.Context,
	userID domainauth.UserID,
	itemID domainitem.ItemID,
	excludeAllocationID domainstorage.AllocationID,
) (int32, int64, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var ownedQuantity int32
	found := false
	for _, allocatedItem := range r.items {
		if allocatedItem.ID == itemID {
			ownedQuantity = allocatedItem.Quantity
			found = true
			break
		}
	}
	if !found {
		return 0, 0, domainitem.ErrItemNotFound
	}

	var total int64
	for _, allocation := range r.allocations {
		if allocation.UserID() != userID || allocation.ItemID() != itemID {
			continue
		}
		if excludeAllocationID != 0 && allocation.ID() == excludeAllocationID {
			continue
		}
		total += int64(allocation.Quantity())
	}
	return ownedQuantity, total, nil
}

func (r *fakeStorageAllocationRepository) LockItemsForUpdate(
	_ context.Context, _ domainauth.UserID, itemIDs []domainitem.ItemID,
) (map[domainitem.ItemID]int32, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	quantities := make(map[domainitem.ItemID]int32, len(itemIDs))
	for _, itemID := range itemIDs {
		for _, allocatedItem := range r.items {
			if allocatedItem.ID == itemID {
				quantities[itemID] = allocatedItem.Quantity
			}
		}
	}
	return quantities, nil
}

func (r *fakeStorageAllocationRepository) FindAllocatedItemByPublicID(
	_ context.Context, _ domainauth.UserID, publicID uuid.UUID,
) (domainstorage.AllocatedItem, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	allocatedItem, ok := r.items[publicID]
	if !ok {
		return domainstorage.AllocatedItem{}, domainitem.ErrItemNotFound
	}
	return allocatedItem, nil
}

func (r *fakeStorageAllocationRepository) ResolveAllocatedItems(
	_ context.Context, _ domainauth.UserID, publicIDs []uuid.UUID,
) ([]domainstorage.AllocatedItem, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	resolved := make([]domainstorage.AllocatedItem, 0, len(publicIDs))
	for _, publicID := range publicIDs {
		allocatedItem, ok := r.items[publicID]
		if !ok {
			return nil, domainitem.ErrItemNotFound
		}
		resolved = append(resolved, allocatedItem)
	}
	return resolved, nil
}

// fakeAuditLogRepository は監査ログを蓄積する。
type fakeAuditLogRepository struct {
	mutex sync.Mutex
	logs  []domainaudit.AuditLog
}

func newFakeAuditLogRepository() *fakeAuditLogRepository {
	return &fakeAuditLogRepository{}
}

var _ domainaudit.AuditLogRepository = (*fakeAuditLogRepository)(nil)

func (r *fakeAuditLogRepository) Create(_ context.Context, log domainaudit.AuditLog) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.logs = append(r.logs, log)
	return nil
}

// actions は記録された操作codeを順に返す。
func (r *fakeAuditLogRepository) actions() []domainaudit.ActionCode {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	actions := make([]domainaudit.ActionCode, 0, len(r.logs))
	for _, log := range r.logs {
		actions = append(actions, log.Action())
	}
	return actions
}

// hasAction は指定操作が記録されたかを返す。
func (r *fakeAuditLogRepository) hasAction(action domainaudit.ActionCode) bool {
	for _, recorded := range r.actions() {
		if recorded == action {
			return true
		}
	}
	return false
}
