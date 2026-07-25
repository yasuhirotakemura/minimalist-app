package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/sqlc"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

type transactionContextKey struct{}

// TransactionManager はpgxのtransactionでtransaction.Managerを実装する。
type TransactionManager struct {
	pool *pgxpool.Pool
}

// NewTransactionManager はTransactionManagerを生成する。
func NewTransactionManager(pool *pgxpool.Pool) *TransactionManager {
	return &TransactionManager{pool: pool}
}

var _ transaction.Manager = (*TransactionManager)(nil)

// WithinTransaction はfnを単一transaction内で実行する。
//
// 既にtransactionが開始済みの場合は、そのtransactionを再利用する。
// これによりApplication Serviceを入れ子に呼び出してもtransaction境界が保たれる。
// panic時はrollbackし、panicを再送出する。
func (m *TransactionManager) WithinTransaction(
	ctx context.Context,
	fn func(ctx context.Context) error,
) error {
	if _, ok := transactionFromContext(ctx); ok {
		return fn(ctx)
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		// rollback失敗はcommit未了の事実を変えないため、原因errorを優先する。
		if rollbackErr := tx.Rollback(context.WithoutCancel(ctx)); rollbackErr != nil &&
			!errors.Is(rollbackErr, pgx.ErrTxClosed) {
			_ = rollbackErr
		}
	}()

	if err := fn(context.WithValue(ctx, transactionContextKey{}, tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	committed = true
	return nil
}

func transactionFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(transactionContextKey{}).(pgx.Tx)
	return tx, ok && tx != nil
}

// Querier はcontextにtransactionがあればそれを、無ければpoolを返す。
//
// Repositoryはこの関数を通してsqlcのQueriesを生成する。
// これによりRepositoryはtransactionの有無を意識せずに実装できる。
func Querier(ctx context.Context, pool *pgxpool.Pool) sqlc.DBTX {
	if tx, ok := transactionFromContext(ctx); ok {
		return tx
	}
	return pool
}
