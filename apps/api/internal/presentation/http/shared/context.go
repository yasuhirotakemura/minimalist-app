// Package shared はHTTP layerの共通処理 (middleware、error変換) を提供する。
package shared

import (
	"context"
	"sync"
)

type requestIDContextKey struct{}
type requestScopeContextKey struct{}

// WithRequestID はrequest IDをcontextへ格納する。
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// RequestIDFromContext はcontextのrequest IDを返す。
func RequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDContextKey{}).(string); ok {
		return requestID
	}
	return ""
}

// RequestScope はaccess logへ出力する値をrequest処理中に収集する。
//
// error codeやユーザーIDは処理の途中で確定するため、mutableな入れ物を用意し
// access log middlewareが最後にまとめて出力する。
type RequestScope struct {
	mutex        sync.Mutex
	errorCode    string
	userPublicID string
}

// WithRequestScope はRequestScopeをcontextへ格納する。
func WithRequestScope(ctx context.Context, scope *RequestScope) context.Context {
	return context.WithValue(ctx, requestScopeContextKey{}, scope)
}

// RequestScopeFromContext はcontextのRequestScopeを返す。
func RequestScopeFromContext(ctx context.Context) (*RequestScope, bool) {
	scope, ok := ctx.Value(requestScopeContextKey{}).(*RequestScope)
	return scope, ok && scope != nil
}

// SetErrorCode はaccess logへ出力するerror codeを記録する。
func (s *RequestScope) SetErrorCode(code string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.errorCode = code
}

// ErrorCode は記録済みのerror codeを返す。
func (s *RequestScope) ErrorCode() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.errorCode
}

// SetUserPublicID はaccess logへ出力するユーザーの公開IDを記録する。
// 内部IDはlogへ出力しない。
func (s *RequestScope) SetUserPublicID(publicID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.userPublicID = publicID
}

// UserPublicID は記録済みのユーザー公開IDを返す。
func (s *RequestScope) UserPublicID() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.userPublicID
}

// RecordErrorCode はcontextにRequestScopeがあればerror codeを記録する。
func RecordErrorCode(ctx context.Context, code string) {
	if scope, ok := RequestScopeFromContext(ctx); ok {
		scope.SetErrorCode(code)
	}
}

// RecordUserPublicID はcontextにRequestScopeがあればユーザー公開IDを記録する。
func RecordUserPublicID(ctx context.Context, publicID string) {
	if scope, ok := RequestScopeFromContext(ctx); ok {
		scope.SetUserPublicID(publicID)
	}
}
