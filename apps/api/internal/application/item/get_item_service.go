package item

import (
	"context"

	"github.com/google/uuid"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
)

// GetItemParams は所持品取得の入力。
type GetItemParams struct {
	UserID   domainauth.UserID
	PublicID uuid.UUID
}

// GetItemResult は所持品取得の結果。
type GetItemResult struct {
	Item ItemResult
}

// GetItemService は所持品を1件取得する。
type GetItemService struct {
	dependencies Dependencies
}

// NewGetItemService はGetItemServiceを生成する。
func NewGetItemService(dependencies Dependencies) *GetItemService {
	return &GetItemService{dependencies: dependencies}
}

// Execute は所持品を取得する。
//
// 参照のみのためtransactionを張らない。
// 他ユーザーのアイテムを指定した場合は ErrItemNotFound を返す (設計書 18.3)。
func (s *GetItemService) Execute(
	ctx context.Context,
	params GetItemParams,
) (GetItemResult, error) {
	found, err := s.dependencies.Items.FindByPublicID(ctx, params.UserID, params.PublicID)
	if err != nil {
		return GetItemResult{}, err
	}
	result, err := s.dependencies.newItemResultWithStorage(ctx, params.UserID, found)
	if err != nil {
		return GetItemResult{}, err
	}
	return GetItemResult{Item: result}, nil
}
