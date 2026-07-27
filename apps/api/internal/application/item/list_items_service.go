package item

import (
	"context"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
)

// ListItemsParams は所持品一覧取得の入力。
type ListItemsParams struct {
	UserID   domainauth.UserID
	Criteria domainitem.ListCriteriaInput
}

// ListItemsService は所持品一覧を取得する。
type ListItemsService struct {
	dependencies Dependencies
}

// NewListItemsService はListItemsServiceを生成する。
func NewListItemsService(dependencies Dependencies) *ListItemsService {
	return &ListItemsService{dependencies: dependencies}
}

// Execute は条件に一致する所持品と総件数を返す。
//
// 参照のみのためtransactionを張らない。
// 総件数は画面のpagination表示に必要なため、一覧と同一条件で別途取得する。
func (s *ListItemsService) Execute(
	ctx context.Context,
	params ListItemsParams,
) (ListItemsResult, error) {
	criteria, err := domainitem.NewListCriteria(params.Criteria)
	if err != nil {
		return ListItemsResult{}, err
	}

	totalCount, err := s.dependencies.Items.Count(ctx, params.UserID, criteria)
	if err != nil {
		return ListItemsResult{}, err
	}

	found, err := s.dependencies.Items.List(ctx, params.UserID, criteria)
	if err != nil {
		return ListItemsResult{}, err
	}

	results := make([]ItemResult, 0, len(found))
	for _, source := range found {
		results = append(results, newItemResult(source))
	}

	return ListItemsResult{
		Items:      results,
		Pagination: newPaginationResult(criteria.Limit, criteria.Offset, totalCount),
	}, nil
}
