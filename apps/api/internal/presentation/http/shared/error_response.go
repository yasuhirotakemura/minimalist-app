package shared

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/logging"
)

// FieldErrorResponse は入力項目単位のerror (設計書 12.3)。
type FieldErrorResponse struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse は全endpoint共通のerror形式 (設計書 12.3)。
type ErrorResponse struct {
	Code        string               `json:"code"`
	Message     string               `json:"message"`
	FieldErrors []FieldErrorResponse `json:"fieldErrors"`
	RequestID   string               `json:"requestId"`
}

// 予期しないerrorを利用者へ返す際の内容。内部情報を含めない (設計書 19.3)。
const (
	internalErrorCode    = "INTERNAL_SERVER_ERROR"
	internalErrorMessage = "サーバーでエラーが発生しました。"
)

// WriteJSON はJSON responseを書き込む。
func WriteJSON(ctx context.Context, w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// header送出後のためstatusは変更できない。logのみ残す。
		logging.FromContext(ctx).Error("failed to encode response body", "error", err.Error())
	}
}

// WriteNoContent は204 responseを書き込む。
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// WriteError はerrorをHTTP responseへ変換する。
//
// DomainErrorのKindからHTTP statusを決定する (設計書 19.2)。
// DomainError以外は500として扱い、詳細はlogへのみ出力する。
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()
	requestID := RequestIDFromContext(ctx)
	logger := logging.FromContext(ctx)

	domainError, ok := shared.AsDomainError(err)
	if !ok {
		RecordErrorCode(ctx, internalErrorCode)
		logger.Error(
			"unhandled error",
			"error", err.Error(),
			logging.KeyRequestID, requestID,
		)
		WriteJSON(ctx, w, http.StatusInternalServerError, ErrorResponse{
			Code:        internalErrorCode,
			Message:     internalErrorMessage,
			FieldErrors: []FieldErrorResponse{},
			RequestID:   requestID,
		})
		return
	}

	status := statusFromKind(domainError.Kind)
	RecordErrorCode(ctx, domainError.Code)

	if status >= http.StatusInternalServerError {
		// 500系は原因を含めてlogへ残すが、responseへは含めない。
		logger.Error(
			"internal error",
			"error", domainError.Error(),
			logging.KeyErrorCode, domainError.Code,
			logging.KeyRequestID, requestID,
		)
		WriteJSON(ctx, w, status, ErrorResponse{
			Code:        internalErrorCode,
			Message:     internalErrorMessage,
			FieldErrors: []FieldErrorResponse{},
			RequestID:   requestID,
		})
		return
	}

	if cause := errors.Unwrap(domainError); cause != nil {
		logger.Debug(
			"request rejected",
			"error", domainError.Error(),
			logging.KeyErrorCode, domainError.Code,
			logging.KeyRequestID, requestID,
		)
	}

	WriteJSON(ctx, w, status, ErrorResponse{
		Code:        domainError.Code,
		Message:     domainError.Message,
		FieldErrors: toFieldErrorResponses(domainError.FieldErrors),
		RequestID:   requestID,
	})
}

func statusFromKind(kind shared.Kind) int {
	switch kind {
	case shared.KindInvalidInput:
		return http.StatusBadRequest
	case shared.KindUnauthenticated:
		return http.StatusUnauthorized
	case shared.KindForbidden:
		return http.StatusForbidden
	case shared.KindNotFound:
		return http.StatusNotFound
	case shared.KindConflict:
		return http.StatusConflict
	case shared.KindRuleViolation:
		return http.StatusUnprocessableEntity
	case shared.KindRateLimited:
		return http.StatusTooManyRequests
	case shared.KindInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

func toFieldErrorResponses(fieldErrors []shared.FieldError) []FieldErrorResponse {
	responses := make([]FieldErrorResponse, 0, len(fieldErrors))
	for _, fieldError := range fieldErrors {
		responses = append(responses, FieldErrorResponse{
			Field:   fieldError.Field,
			Code:    fieldError.Code,
			Message: fieldError.Message,
		})
	}
	return responses
}
