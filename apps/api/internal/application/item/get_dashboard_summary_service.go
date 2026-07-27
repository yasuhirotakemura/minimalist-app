package item

import (
	"context"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
)

// GetDashboardSummaryParams はダッシュボード集計取得の入力。
type GetDashboardSummaryParams struct {
	UserID domainauth.UserID
}

// GetDashboardSummaryService はダッシュボードの集計値を取得する (設計書 9.3)。
type GetDashboardSummaryService struct {
	dependencies Dependencies
}

// NewGetDashboardSummaryService はGetDashboardSummaryServiceを生成する。
func NewGetDashboardSummaryService(dependencies Dependencies) *GetDashboardSummaryService {
	return &GetDashboardSummaryService{dependencies: dependencies}
}

// Execute は集計値を返す。
//
// 参照のみのためtransactionを張らない。
// 内訳の表示順の決定はDomain (item.NewSummary) が行う。
func (s *GetDashboardSummaryService) Execute(
	ctx context.Context,
	params GetDashboardSummaryParams,
) (DashboardSummaryResult, error) {
	totals, err := s.dependencies.Items.AggregateSummary(ctx, params.UserID)
	if err != nil {
		return DashboardSummaryResult{}, err
	}
	return newDashboardSummaryResult(domainitem.NewSummary(totals)), nil
}
