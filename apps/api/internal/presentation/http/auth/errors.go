package auth

import "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"

// errAuthenticationMiddlewareMissing はrouterの設定不備を表す。
// 到達した場合は500として扱い、利用者へは詳細を返さない。
//
// 他のfeature handlerと同じerrorを返すため、定義はshared packageへ置く。
var errAuthenticationMiddlewareMissing = shared.ErrAuthenticationMiddlewareMissing
