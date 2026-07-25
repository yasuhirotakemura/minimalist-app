// Package shared はdomain全体で共有するerror表現を提供する。
//
// DomainErrorはHTTP status codeを持たない。HTTPへの変換は
// presentation layerが Kind を見て行う (設計書 3.3 / 19.2)。
package shared

import (
	"errors"
	"fmt"
)

// Kind はerrorの分類。presentation layerがHTTP statusへ変換する。
type Kind int

const (
	// KindInvalidInput はrequest形式が不正であることを表す。
	KindInvalidInput Kind = iota
	// KindUnauthenticated は認証されていないことを表す。
	KindUnauthenticated
	// KindForbidden は認証済みだが操作が許可されないことを表す。
	KindForbidden
	// KindNotFound は対象が存在しないことを表す。
	KindNotFound
	// KindConflict はversion競合・unique競合・状態競合を表す。
	KindConflict
	// KindRuleViolation は業務ルール違反を表す。
	KindRuleViolation
	// KindRateLimited はrequest回数の上限超過を表す。
	KindRateLimited
	// KindInternal は予期しないerrorを表す。
	KindInternal
)

// String はKindの名称を返す。
func (k Kind) String() string {
	switch k {
	case KindInvalidInput:
		return "invalid_input"
	case KindUnauthenticated:
		return "unauthenticated"
	case KindForbidden:
		return "forbidden"
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	case KindRuleViolation:
		return "rule_violation"
	case KindRateLimited:
		return "rate_limited"
	case KindInternal:
		return "internal"
	default:
		return "unknown"
	}
}

// FieldError は入力項目単位のerror。
type FieldError struct {
	Field   string
	Code    string
	Message string
}

// NewFieldError はFieldErrorを生成する。
func NewFieldError(field, code, message string) FieldError {
	return FieldError{Field: field, Code: code, Message: message}
}

// DomainError は業務上意味のあるerror。
//
// Messageは利用者向けの文言とし、内部実装の詳細を含めない (設計書 19.3)。
type DomainError struct {
	Kind        Kind
	Code        string
	Message     string
	FieldErrors []FieldError
	cause       error
}

// Error はerror interfaceを満たす。
func (e *DomainError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap は原因errorを返す。
func (e *DomainError) Unwrap() error {
	return e.cause
}

// Is はCodeが一致するDomainErrorを同一として扱う。
// これによりsentinel errorとerrors.Isを併用できる。
func (e *DomainError) Is(target error) bool {
	var other *DomainError
	if !errors.As(target, &other) {
		return false
	}
	return e.Code == other.Code
}

// WithCause は原因errorを付与した複製を返す。
func (e *DomainError) WithCause(cause error) *DomainError {
	copied := *e
	copied.cause = cause
	return &copied
}

// WithFieldErrors はfield errorを付与した複製を返す。
func (e *DomainError) WithFieldErrors(fieldErrors ...FieldError) *DomainError {
	copied := *e
	copied.FieldErrors = append(append([]FieldError{}, e.FieldErrors...), fieldErrors...)
	return &copied
}

func newDomainError(kind Kind, code, message string) *DomainError {
	return &DomainError{Kind: kind, Code: code, Message: message}
}

// NewInvalidInputError はrequest形式不正のerrorを生成する。
func NewInvalidInputError(code, message string) *DomainError {
	return newDomainError(KindInvalidInput, code, message)
}

// NewUnauthenticatedError は未認証のerrorを生成する。
func NewUnauthenticatedError(code, message string) *DomainError {
	return newDomainError(KindUnauthenticated, code, message)
}

// NewForbiddenError は操作不可のerrorを生成する。
func NewForbiddenError(code, message string) *DomainError {
	return newDomainError(KindForbidden, code, message)
}

// NewNotFoundError は対象不存在のerrorを生成する。
func NewNotFoundError(code, message string) *DomainError {
	return newDomainError(KindNotFound, code, message)
}

// NewConflictError は競合のerrorを生成する。
func NewConflictError(code, message string) *DomainError {
	return newDomainError(KindConflict, code, message)
}

// NewRuleViolationError は業務ルール違反のerrorを生成する。
func NewRuleViolationError(code, message string) *DomainError {
	return newDomainError(KindRuleViolation, code, message)
}

// NewRateLimitedError はrate limit超過のerrorを生成する。
func NewRateLimitedError(code, message string) *DomainError {
	return newDomainError(KindRateLimited, code, message)
}

// NewInternalError は予期しないerrorを生成する。
func NewInternalError(code, message string) *DomainError {
	return newDomainError(KindInternal, code, message)
}

// AsDomainError はerror chainからDomainErrorを取り出す。
func AsDomainError(err error) (*DomainError, bool) {
	var domainError *DomainError
	if errors.As(err, &domainError) {
		return domainError, true
	}
	return nil, false
}
