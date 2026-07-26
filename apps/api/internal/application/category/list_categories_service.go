// Package category はカテゴリーのユースケースを実装する。
//
// 既定カテゴリーの作成はユーザー登録ユースケースの一部として
// application/auth の RegisterUserService が同一transaction内で行う。
// 本packageは参照系のみを提供する。
//
// カテゴリーの登録・更新・archive・並び替え (設計書 12.4) は
// 今回のスコープ外のため実装しない。
package category

import (
	"context"
	"time"

	"github.com/google/uuid"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domaincategory "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/category"
)

// Dependencies はカテゴリーユースケースが必要とする依存をまとめる。
type Dependencies struct {
	Categories domaincategory.CategoryRepository
}

// CategoryResult はユースケースが返すカテゴリーの表現。内部IDを含めない。
type CategoryResult struct {
	PublicID    uuid.UUID
	Name        string
	Description *string
	SortOrder   int32
	Version     int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListCategoriesParams はカテゴリー一覧取得の入力。
type ListCategoriesParams struct {
	UserID domainauth.UserID
}

// ListCategoriesResult はカテゴリー一覧の結果。
type ListCategoriesResult struct {
	Categories []CategoryResult
}

// ListCategoriesService はカテゴリー一覧を取得する。
type ListCategoriesService struct {
	dependencies Dependencies
}

// NewListCategoriesService はListCategoriesServiceを生成する。
func NewListCategoriesService(dependencies Dependencies) *ListCategoriesService {
	return &ListCategoriesService{dependencies: dependencies}
}

// Execute は認証ユーザーのカテゴリーを表示順で返す。
func (s *ListCategoriesService) Execute(
	ctx context.Context,
	params ListCategoriesParams,
) (ListCategoriesResult, error) {
	found, err := s.dependencies.Categories.ListActiveByUserID(ctx, params.UserID)
	if err != nil {
		return ListCategoriesResult{}, err
	}

	results := make([]CategoryResult, 0, len(found))
	for _, source := range found {
		results = append(results, CategoryResult{
			PublicID:    source.PublicID(),
			Name:        source.Name(),
			Description: source.Description(),
			SortOrder:   source.SortOrder(),
			Version:     source.Version(),
			CreatedAt:   source.CreatedAt(),
			UpdatedAt:   source.UpdatedAt(),
		})
	}

	return ListCategoriesResult{Categories: results}, nil
}
