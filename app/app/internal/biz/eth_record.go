package biz

import (
	"context"
	"time"
)

// EthUserRecord 链上充值记录（对齐 new18new eth_user_record）.
type EthUserRecord struct {
	ID        int64
	UserId    int64
	Hash      string
	Status    string
	Type      string
	Amount    string
	AmountTwo uint64
	RelAmount int64
	CoinType  string
	Last      int64
	CreatedAt time.Time
}

// EthUserRecordRepo 游标与入账记录.
type EthUserRecordRepo interface {
	GetEthUserRecordLast(ctx context.Context) (int64, error)
	GetByLast(ctx context.Context, last int64) (*EthUserRecord, error)
	CreateEthUserRecordListByHash(ctx context.Context, r *EthUserRecord) (*EthUserRecord, error)
	DeleteByLast(ctx context.Context, last int64) error
}
