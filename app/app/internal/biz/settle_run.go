package biz

import (
	"context"
	"time"
)

// SettleRun is one daily settlement marker (claim and/or completed counts).
type SettleRun struct {
	ID               uint64
	SettleDate       time.Time // date only
	Forced           bool
	SettledCount     int
	ExitedCount      int
	GenerationCount  int
	CommunityCount   int
	PeerCount        int
	Remark           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// SettleRunRepo persists settle_runs for same-day idempotency / multi-instance claim.
type SettleRunRepo interface {
	FindByDate(ctx context.Context, settleDate time.Time) (*SettleRun, error)
	// FindLatest returns the most recent settle_runs row (by settle_date), or nil.
	FindLatest(ctx context.Context) (*SettleRun, error)
	// TryClaim inserts a placeholder row for the calendar day.
	// claimed=false means another instance already owns the day (unique conflict).
	TryClaim(ctx context.Context, settleDate time.Time, remark string) (claimed bool, err error)
	Upsert(ctx context.Context, run *SettleRun) error
}
