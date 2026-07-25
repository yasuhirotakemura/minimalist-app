package auth

import domainshared "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"

// errAuthenticationMiddlewareMissing はrouterの設定不備を表す。
// 到達した場合は500として扱い、利用者へは詳細を返さない。
var errAuthenticationMiddlewareMissing = domainshared.NewInternalError(
	"AUTHENTICATION_MIDDLEWARE_MISSING",
	"サーバーでエラーが発生しました。",
)
