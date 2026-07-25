package shared

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	domainshared "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// MaxJSONRequestBodyBytes はJSON request bodyの上限 (設計書 24.14)。
// Phase 0のendpointは小さなJSONのみを受け取る。
const MaxJSONRequestBodyBytes = 64 * 1024

// ErrBadRequest はrequest形式が不正であることを表す。
var ErrBadRequest = domainshared.NewInvalidInputError(
	"BAD_REQUEST",
	"リクエストの形式が正しくありません。",
)

// DecodeJSONBody はrequest bodyをJSONとして読み取る。
//
// 未知のfieldを拒否し、OpenAPIのadditionalProperties:falseと挙動を揃える。
// bodyの大きさを制限し、巨大なrequestによる資源消費を防ぐ。
func DecodeJSONBody(w http.ResponseWriter, r *http.Request, target any) error {
	contentType := r.Header.Get("Content-Type")
	if contentType != "" {
		mediaType, _, _ := strings.Cut(contentType, ";")
		if strings.TrimSpace(strings.ToLower(mediaType)) != "application/json" {
			return ErrBadRequest.WithFieldErrors(domainshared.NewFieldError(
				"", "UNSUPPORTED_MEDIA_TYPE", "Content-Type は application/json を指定してください。"))
		}
	}

	limited := http.MaxBytesReader(w, r.Body, MaxJSONRequestBodyBytes)
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return toDecodeError(err)
	}

	// bodyへ複数のJSON値が含まれていないことを確認する。
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrBadRequest.WithFieldErrors(domainshared.NewFieldError(
			"", "MALFORMED_JSON", "リクエストボディの形式が正しくありません。"))
	}

	return nil
}

func toDecodeError(err error) error {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		return ErrBadRequest.WithFieldErrors(domainshared.NewFieldError(
			"", "PAYLOAD_TOO_LARGE", "リクエストボディが大きすぎます。")).WithCause(err)
	}

	var unmarshalTypeError *json.UnmarshalTypeError
	if errors.As(err, &unmarshalTypeError) {
		return ErrBadRequest.WithFieldErrors(domainshared.NewFieldError(
			unmarshalTypeError.Field, "INVALID_TYPE", "値の型が正しくありません。")).WithCause(err)
	}

	if errors.Is(err, io.EOF) {
		return ErrBadRequest.WithFieldErrors(domainshared.NewFieldError(
			"", "REQUIRED", "リクエストボディを指定してください。")).WithCause(err)
	}

	return ErrBadRequest.WithFieldErrors(domainshared.NewFieldError(
		"", "MALFORMED_JSON", "リクエストボディの形式が正しくありません。")).WithCause(err)
}
