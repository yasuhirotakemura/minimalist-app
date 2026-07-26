package shared

import (
	"errors"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/openapi"

	domainshared "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// ErrAuthenticationMiddlewareMissing はrouterの設定不備を表す。
//
// 認証必須のrouteへ RequireAuthenticatedUser middleware を適用し忘れた場合に
// handlerが到達する。500として扱い、利用者へは詳細を返さない。
var ErrAuthenticationMiddlewareMissing = domainshared.NewInternalError(
	"AUTHENTICATION_MIDDLEWARE_MISSING",
	"サーバーでエラーが発生しました。",
)

// NewParameterBindingError はpath・query parameterの解釈失敗をDomainErrorへ変換する。
//
// OpenAPI生成のwrapperが返すerrorから対象parameter名を取り出し、
// 共通のerror形式 (設計書 12.3) で返せるようにする。
func NewParameterBindingError(err error) error {
	message := "リクエストパラメータの形式が正しくありません。"

	var requiredParam *openapi.RequiredParamError
	if errors.As(err, &requiredParam) {
		return ErrBadRequest.WithFieldErrors(domainshared.NewFieldError(
			requiredParam.ParamName, "REQUIRED", "必須のパラメータが指定されていません。")).
			WithCause(err)
	}

	var invalidFormat *openapi.InvalidParamFormatError
	if errors.As(err, &invalidFormat) {
		return ErrBadRequest.WithFieldErrors(domainshared.NewFieldError(
			invalidFormat.ParamName, "INVALID_FORMAT", message)).WithCause(err)
	}

	var tooManyValues *openapi.TooManyValuesForParamError
	if errors.As(err, &tooManyValues) {
		return ErrBadRequest.WithFieldErrors(domainshared.NewFieldError(
			tooManyValues.ParamName, "TOO_MANY_VALUES",
			"同じパラメータを複数指定することはできません。")).WithCause(err)
	}

	return ErrBadRequest.WithFieldErrors(
		domainshared.NewFieldError("", "INVALID_FORMAT", message)).WithCause(err)
}
