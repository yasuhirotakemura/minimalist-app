// Package idgenerator は外部公開ID (public_id) の生成を抽象化する。
// 内部主キーはDBのIDENTITYが払い出すため、ここでは扱わない (設計書 4.3)。
package idgenerator

import "github.com/google/uuid"

// PublicIDGenerator は外部公開IDを生成する。
type PublicIDGenerator interface {
	NewPublicID() (uuid.UUID, error)
}

// UUIDv7Generator は時刻順序を持つUUIDv7を生成する。
// index局所性が高く、B-treeの断片化を抑えられる。
type UUIDv7Generator struct{}

// NewUUIDv7Generator はUUIDv7Generatorを返す。
func NewUUIDv7Generator() UUIDv7Generator {
	return UUIDv7Generator{}
}

// NewPublicID はUUIDv7を生成する。
func (UUIDv7Generator) NewPublicID() (uuid.UUID, error) {
	return uuid.NewV7()
}
