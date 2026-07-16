package biz

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

const (
	LedgerStatic        = "static"
	LedgerGeneration    = "generation"
	LedgerCommunityBase = "community_base"
	LedgerPeer          = "peer"
	LedgerExtract       = "extract"
	LedgerDeposit       = "deposit"
	LedgerAdminAdjust   = "admin_adjust"
	BalanceWithdrawable = "withdrawable"
	BalanceAccount      = "account"
)

// LedgerEntry is one earnings / balance movement record.
type LedgerEntry struct {
	ID          uint64
	UserID      uint64
	OrderID     *uint64
	EntryType   string
	Amount      decimal.Decimal
	BalanceKind string
	Remark      string
	CreatedAt   time.Time
}

// LedgerRepo persists ledger entries.
type LedgerRepo interface {
	Create(ctx context.Context, e *LedgerEntry) error
	ListByUser(ctx context.Context, userID uint64, from, to time.Time) ([]*LedgerEntry, error)
	ListPaged(ctx context.Context, address, entryType string, page, pageSize int) ([]*LedgerEntry, int, error)
	SumAmountByTypes(ctx context.Context, entryTypes []string) (decimal.Decimal, error)
	SumAmountByTypesBetween(ctx context.Context, entryTypes []string, from, to time.Time) (decimal.Decimal, error)
}
