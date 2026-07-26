package item

import (
	"context"

	"github.com/google/uuid"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
)

// ListItemUsageRecordsParams は使用記録履歴取得の入力。
type ListItemUsageRecordsParams struct {
	UserID   domainauth.UserID
	PublicID uuid.UUID
	Limit    *int32
	Offset   *int32
}

// ListItemUsageRecordsService は使用記録の履歴を取得する。
type ListItemUsageRecordsService struct {
	dependencies Dependencies
}

// NewListItemUsageRecordsService はListItemUsageRecordsServiceを生成する。
func NewListItemUsageRecordsService(dependencies Dependencies) *ListItemUsageRecordsService {
	return &ListItemUsageRecordsService{dependencies: dependencies}
}

// Execute は使用記録の履歴を使用日時の降順で返す。
//
// アイテムの所有者確認を兼ねて先にアイテムを取得する。
// 他ユーザーのアイテムを指定した場合は ErrItemNotFound を返す (設計書 18.3)。
func (s *ListItemUsageRecordsService) Execute(
	ctx context.Context,
	params ListItemUsageRecordsParams,
) (ListUsageRecordsResult, error) {
	page, err := domainitem.NewPageCriteria(params.Limit, params.Offset)
	if err != nil {
		return ListUsageRecordsResult{}, err
	}

	owner, err := s.dependencies.Items.FindByPublicID(ctx, params.UserID, params.PublicID)
	if err != nil {
		return ListUsageRecordsResult{}, err
	}

	totalCount, err := s.dependencies.UsageRecords.CountByItemID(
		ctx, params.UserID, owner.ID())
	if err != nil {
		return ListUsageRecordsResult{}, err
	}

	records, err := s.dependencies.UsageRecords.ListByItemID(
		ctx, params.UserID, owner.ID(), page)
	if err != nil {
		return ListUsageRecordsResult{}, err
	}

	results := make([]UsageRecordResult, 0, len(records))
	for _, record := range records {
		results = append(results, newUsageRecordResult(record))
	}

	return ListUsageRecordsResult{
		Items:      results,
		Pagination: newPaginationResult(page.Limit, page.Offset, totalCount),
	}, nil
}
