// Package postgresql はDomain RepositoryのPostgreSQL実装を提供する。
//
// SQLは sql/queries/ へ置き、sqlcで生成したcodeのみを使用する (設計書 11.6)。
// DB rowとDomain Entityは本packageで相互変換し、Domainへpgx固有型を漏らさない。
package postgresql

import (
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// timestamptz は時刻をUTCでpgtype.Timestamptzへ変換する (設計書 4.3)。
func timestamptz(instant time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: instant.UTC(), Valid: true}
}

// optionalTime はpgtype.Timestamptzを*time.Timeへ変換する。
func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	instant := value.Time.UTC()
	return &instant
}

// utcTime はpgtype.Timestamptzをtime.Timeへ変換する。NULLはzero valueとする。
func utcTime(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time.UTC()
}

// optionalString は空文字をNULLとして扱うためのpointerへ変換する。
func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// stringValue は*stringを文字列へ変換する。NULLは空文字とする。
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// optionalIPAddress は文字列をinet columnへ渡す値へ変換する。
//
// 解析できない値はNULLとして扱う。IPアドレスの記録は監査補助であり、
// 記録できないことを理由にlogin自体を失敗させない。
func optionalIPAddress(value string) *netip.Addr {
	if value == "" {
		return nil
	}
	address, err := netip.ParseAddr(value)
	if err != nil {
		return nil
	}
	return &address
}

// ipAddressValue はinet columnの値を文字列へ変換する。NULLは空文字とする。
func ipAddressValue(value *netip.Addr) string {
	if value == nil || !value.IsValid() {
		return ""
	}
	return value.String()
}
