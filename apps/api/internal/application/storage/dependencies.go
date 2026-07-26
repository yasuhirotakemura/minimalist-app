// Package storage は収納単位・収納割当のユースケースを実装する。
//
// Application Serviceの責務 (設計書 11.4):
//   - Repository呼出
//   - Entity生成 / ValueObject生成
//   - transaction制御
//   - 複数処理の順序制御
//   - Domain errorの伝播
//   - audit log記録
//
// HTTP status codeやSQLをここへ書かない。
package storage

import (
	"context"

	"github.com/google/uuid"

	applicationaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
)

// Dependencies は収納ユースケースが必要とする依存をまとめる。
type Dependencies struct {
	StorageUnits  domainstorage.StorageUnitRepository
	Allocations   domainstorage.StorageAllocationRepository
	AuditRecorder *applicationaudit.Recorder
}

// AttributesParams は利用者が指定できる収納単位の属性。
//
// codeは文字列のまま受け取り、ValueObjectへの変換はDomainが行う。
// これによりpresentation layerがDomainのcode体系へ依存しない。
type AttributesParams struct {
	Name                      string
	StorageTypeCode           string
	MobilityClassCode         string
	ParentStorageUnitPublicID *uuid.UUID
	TareWeightGram            *int32
	MaximumWeightGram         *int32
	MaximumVolumeMilliliter   *int32
	Description               *string
	SortOrder                 *int32
}

// toDomainAttributes はDomainの属性を組み立てる。親は別途解決する。
func (p AttributesParams) toDomainAttributes() domainstorage.Attributes {
	attributes := domainstorage.Attributes{
		Name:                    p.Name,
		StorageType:             domainstorage.StorageType(p.StorageTypeCode),
		MobilityClass:           domainitem.MobilityClass(p.MobilityClassCode),
		TareWeightGram:          p.TareWeightGram,
		MaximumWeightGram:       p.MaximumWeightGram,
		MaximumVolumeMilliliter: p.MaximumVolumeMilliliter,
		Description:             p.Description,
	}
	if p.SortOrder != nil {
		attributes.SortOrder = *p.SortOrder
	}
	return attributes
}

// resolveParent は親収納単位を解決する。
//
// 未指定の場合はnilを返し、rootとして扱う。
// 存在しない親、および他ユーザーの収納単位を指定した場合は
// ErrStorageUnitNotFound を返す (設計書 18.3)。
func (d Dependencies) resolveParent(
	ctx context.Context,
	userID domainauth.UserID,
	parentPublicID *uuid.UUID,
) (*domainstorage.StorageUnit, error) {
	if parentPublicID == nil {
		return nil, nil
	}

	parent, err := d.StorageUnits.FindByPublicID(ctx, userID, *parentPublicID)
	if err != nil {
		return nil, err
	}
	return &parent, nil
}

// hierarchySnapshot はユーザーの収納単位の木と、収納単位ごとの直接割当。
//
// 容量集計・部分木の高さ算出・子件数の取得は木全体を必要とするため、
// 1度の読み込みを使い回す。
type hierarchySnapshot struct {
	units       []domainstorage.StorageUnit
	allocations map[domainstorage.StorageUnitID][]domainstorage.StorageAllocation
	capacities  map[domainstorage.StorageUnitID]domainstorage.Capacity
}

// loadHierarchy は容量集計に必要な木と割当をまとめて読み込む。
//
// archive済みも読み込む。archive済み収納単位の詳細画面でも
// 集計値を表示するためであり、親への合算からは除外される。
func (d Dependencies) loadHierarchy(
	ctx context.Context,
	userID domainauth.UserID,
) (hierarchySnapshot, error) {
	units, err := d.StorageUnits.ListAll(ctx, userID, true)
	if err != nil {
		return hierarchySnapshot{}, err
	}

	unitIDs := make([]domainstorage.StorageUnitID, 0, len(units))
	for _, unit := range units {
		unitIDs = append(unitIDs, unit.ID())
	}

	allocations, err := d.Allocations.ListByStorageUnitIDs(ctx, userID, unitIDs)
	if err != nil {
		return hierarchySnapshot{}, err
	}

	return hierarchySnapshot{
		units:       units,
		allocations: allocations,
		capacities:  domainstorage.CalculateHierarchyCapacities(units, allocations),
	}, nil
}

// capacityOf は指定収納単位の集計を返す。木に含まれない場合は空の集計を返す。
func (s hierarchySnapshot) capacityOf(id domainstorage.StorageUnitID) domainstorage.Capacity {
	return s.capacities[id]
}

// subtreeHeight は指定収納単位を根とする部分木の高さを返す。
//
// 子を持たない場合は1。収納単位を別の親の下へ移動する際、
// 配下ごと移動しても階層上限を超えないことを検証するために使用する。
// archive済みの子は復元されうるため高さに数える。
func (s hierarchySnapshot) subtreeHeight(id domainstorage.StorageUnitID) int32 {
	childIDs := make(map[domainstorage.StorageUnitID][]domainstorage.StorageUnitID, len(s.units))
	for _, unit := range s.units {
		if unit.HasParent() {
			parentID := unit.Parent().ID
			childIDs[parentID] = append(childIDs[parentID], unit.ID())
		}
	}
	return heightOf(id, childIDs)
}

// heightOf は再帰で部分木の高さを求める。
//
// 階層上限が3のため再帰は高々3段で終わる。
func heightOf(
	id domainstorage.StorageUnitID,
	childIDs map[domainstorage.StorageUnitID][]domainstorage.StorageUnitID,
) int32 {
	height := int32(1)
	for _, childID := range childIDs[id] {
		childHeight := heightOf(childID, childIDs) + 1
		if childHeight > height {
			height = childHeight
		}
	}
	return height
}

// assignedQuantitiesOf はアイテムごとの全収納単位への割当数量合計を返す。
//
// 収納内容編集画面が「他収納への割当数量」と「未割当数量」を表示するために使用する。
func (d Dependencies) assignedQuantitiesOf(
	ctx context.Context,
	userID domainauth.UserID,
	itemIDs []domainitem.ItemID,
) (map[domainitem.ItemID]int64, error) {
	totals := make(map[domainitem.ItemID]int64, len(itemIDs))
	if len(itemIDs) == 0 {
		return totals, nil
	}

	allocationsByItemID, err := d.Allocations.ListByItemIDs(ctx, userID, itemIDs)
	if err != nil {
		return nil, err
	}
	for itemID, allocations := range allocationsByItemID {
		for _, allocation := range allocations {
			totals[itemID] += int64(allocation.Quantity())
		}
	}
	return totals, nil
}

// buildContents は収納単位の内容 (割当・子収納単位・集計) を組み立てる。
func (d Dependencies) buildContents(
	ctx context.Context,
	userID domainauth.UserID,
	unit domainstorage.StorageUnit,
) (StorageUnitContentsResult, error) {
	snapshot, err := d.loadHierarchy(ctx, userID)
	if err != nil {
		return StorageUnitContentsResult{}, err
	}

	allocations := snapshot.allocations[unit.ID()]
	itemIDs := make([]domainitem.ItemID, 0, len(allocations))
	for _, allocation := range allocations {
		itemIDs = append(itemIDs, allocation.ItemID())
	}
	assignedQuantities, err := d.assignedQuantitiesOf(ctx, userID, itemIDs)
	if err != nil {
		return StorageUnitContentsResult{}, err
	}

	allocationResults := make([]StorageAllocationResult, 0, len(allocations))
	for _, allocation := range allocations {
		allocationResults = append(allocationResults,
			newAllocationResult(allocation, assignedQuantities[allocation.ItemID()]))
	}

	children := make([]StorageUnitResult, 0)
	for _, candidate := range snapshot.units {
		if candidate.IsArchived() || !candidate.HasParent() {
			continue
		}
		if candidate.Parent().ID != unit.ID() {
			continue
		}
		children = append(children,
			newStorageUnitResult(candidate, snapshot.capacityOf(candidate.ID())))
	}

	return StorageUnitContentsResult{
		StorageUnit:       newStorageUnitResult(unit, snapshot.capacityOf(unit.ID())),
		Allocations:       allocationResults,
		ChildStorageUnits: children,
	}, nil
}
