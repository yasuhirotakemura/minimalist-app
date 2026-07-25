// Package transaction はtransaction境界の抽象を提供する。
//
// Application Serviceはtransaction境界を制御するが、PostgreSQL固有の型へ
// 依存してはならない (設計書 3.3 / 11.4)。そのためinterfaceのみをここへ置き、
// 実装は infrastructure/postgresql へ置く。
package transaction

import "context"

// Manager はfnを単一transaction内で実行する。
// fnがerrorを返した場合はrollbackし、そのerrorを返す。
// 呼び出し先のRepositoryは、渡されたcontextからtransactionを取得する。
type Manager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// PassthroughManager はtransactionを張らずにfnを実行する。unit test用。
type PassthroughManager struct{}

// NewPassthroughManager はPassthroughManagerを返す。
func NewPassthroughManager() PassthroughManager {
	return PassthroughManager{}
}

// WithinTransaction はfnをそのまま実行する。
func (PassthroughManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
