package biz

import (
	"context"
	"strings"
	"sync"

	"github.com/jinniu/app/app/app/internal/pkg/payout"
	"github.com/shopspring/decimal"
)

// PayoutConfig holds on-chain withdraw payout settings (ADR 0010).
type PayoutConfig struct {
	Enabled      bool
	RPC          string
	USDT         string
	HotWalletKey string
	// MaxUSDT when >0 rejects payout if credited_amount > MaxUSDT (env JINNIU_PAYOUT_MAX_USDT).
	MaxUSDT decimal.Decimal
	// CronExpr empty means no scheduled payout (manual / admin only).
	CronExpr string
}

// SetPayoutConfig wires payout settings from composition root.
func (uc *RecordUseCase) SetPayoutConfig(cfg PayoutConfig) {
	uc.payoutCfg = cfg
}

var payoutMu sync.Mutex // process-wide: one payout attempt at a time

// ProcessWithdrawPayout claims a rewarded withdraw and broadcasts USDT (or confirms existing tx_hash).
func (uc *RecordUseCase) ProcessWithdrawPayout(ctx context.Context, id uint64) (*Withdraw, error) {
	payoutMu.Lock()
	defer payoutMu.Unlock()

	if !uc.payoutCfg.Enabled {
		return nil, ErrPayoutDisabled
	}
	if !uc.payoutCfg.MaxUSDT.IsPositive() {
		return nil, ErrPayoutMaxRequired
	}
	if uc.payoutCfg.RPC == "" || uc.payoutCfg.USDT == "" || uc.payoutCfg.HotWalletKey == "" {
		return nil, ErrPayoutNotConfigured
	}

	w, err := uc.withdraws.FindByID(ctx, id)
	if err != nil || w == nil {
		return nil, ErrWithdrawNotFound
	}

	// Already paid
	if w.Status == WithdrawPass {
		return w, nil
	}

	// Confirm path: doing with hash
	if w.Status == WithdrawDoing && w.TxHash != "" {
		return uc.confirmPayout(ctx, w)
	}

	// Safety cap before claim (S1 / L1)
	if uc.payoutCfg.MaxUSDT.IsPositive() && w.CreditedAmount.GreaterThan(uc.payoutCfg.MaxUSDT) {
		return nil, ErrPayoutAboveMax
	}

	// Claim if still in queue
	if w.Status == WithdrawRewarded || w.Status == WithdrawApproved {
		ok, err := uc.withdraws.CasClaimPayout(ctx, id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrWithdrawConflict
		}
		w, err = uc.withdraws.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	if w.Status != WithdrawDoing {
		return nil, ErrWithdrawConflict
	}

	// Has hash already (crash after broadcast): only confirm
	if w.TxHash != "" {
		return uc.confirmPayout(ctx, w)
	}

	user, err := uc.users.FindByID(ctx, w.UserID)
	if err != nil || user == nil {
		_ = uc.withdraws.SetPayoutError(ctx, id, "user not found")
		return nil, ErrUserNotFound
	}

	hash, err := payout.TransferUSDT(ctx, uc.payoutCfg.RPC, uc.payoutCfg.USDT, uc.payoutCfg.HotWalletKey, user.Address, w.CreditedAmount)
	if err != nil {
		_ = uc.withdraws.SetPayoutError(ctx, id, err.Error())
		return nil, err
	}
	if err := uc.withdraws.SetTxHash(ctx, id, hash); err != nil {
		return nil, err
	}
	w.TxHash = hash

	return uc.confirmPayout(ctx, w)
}

func (uc *RecordUseCase) confirmPayout(ctx context.Context, w *Withdraw) (*Withdraw, error) {
	mined, success, err := payout.ReceiptOK(ctx, uc.payoutCfg.RPC, w.TxHash)
	if err != nil {
		_ = uc.withdraws.SetPayoutError(ctx, w.ID, err.Error())
		return nil, err
	}
	if !mined {
		_ = uc.withdraws.SetPayoutError(ctx, w.ID, "tx not mined yet")
		w.PayoutError = "tx not mined yet"
		return w, nil
	}
	if !success {
		_ = uc.withdraws.SetPayoutError(ctx, w.ID, "tx reverted")
		return nil, ErrWithdrawConflict
	}
	ok, err := uc.withdraws.MarkPass(ctx, w.ID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return uc.withdraws.FindByID(ctx, w.ID)
	}
	w.Status = WithdrawPass
	w.PayoutError = ""
	return w, nil
}

// RunPayoutQueue processes up to limit withdraws in the payout queue.
func (uc *RecordUseCase) RunPayoutQueue(ctx context.Context, limit int) (done int, err error) {
	if !uc.payoutCfg.Enabled {
		return 0, ErrPayoutDisabled
	}
	if !uc.payoutCfg.MaxUSDT.IsPositive() {
		return 0, ErrPayoutMaxRequired
	}
	list, err := uc.withdraws.ListPayoutQueue(ctx, limit)
	if err != nil {
		return 0, err
	}
	for _, w := range list {
		if _, e := uc.ProcessWithdrawPayout(ctx, w.ID); e == nil {
			done++
		}
	}
	return done, nil
}

// ConfirmWithdrawPayout only checks receipt for doing+tx_hash.
func (uc *RecordUseCase) ConfirmWithdrawPayout(ctx context.Context, id uint64) (*Withdraw, error) {
	payoutMu.Lock()
	defer payoutMu.Unlock()
	w, err := uc.withdraws.FindByID(ctx, id)
	if err != nil || w == nil {
		return nil, ErrWithdrawNotFound
	}
	if w.TxHash == "" {
		return nil, ErrWithdrawConflict
	}
	if w.Status == WithdrawPass {
		return w, nil
	}
	if w.Status != WithdrawDoing {
		return nil, ErrWithdrawConflict
	}
	return uc.confirmPayout(ctx, w)
}

// PayoutStatusView is admin ops snapshot (no on-chain balance).
type PayoutStatusView struct {
	Enabled          bool
	KeyConfigured    bool
	HotAddress       string
	MaxUSDT          string // empty if no cap
	PayoutCron       string
	QueueRewarded    int64
	QueueDoing       int64
	AllowForceSettle bool
	MaxRequiredOK    bool // payout off, or max > 0 when enabled
}

// PayoutStatus returns payout configuration + queue counts (no chain balance).
func (uc *RecordUseCase) PayoutStatus(ctx context.Context) (*PayoutStatusView, error) {
	view := &PayoutStatusView{
		Enabled:          uc.payoutCfg.Enabled,
		KeyConfigured:    strings.TrimSpace(uc.payoutCfg.HotWalletKey) != "",
		PayoutCron:       uc.payoutCfg.CronExpr,
		AllowForceSettle: uc.allowForceSettle,
		MaxRequiredOK:    !uc.payoutCfg.Enabled || uc.payoutCfg.MaxUSDT.IsPositive(),
	}
	if uc.payoutCfg.MaxUSDT.IsPositive() {
		view.MaxUSDT = uc.payoutCfg.MaxUSDT.String()
	}
	if view.KeyConfigured {
		if addr, err := payout.HotAddress(uc.payoutCfg.HotWalletKey); err == nil {
			view.HotAddress = addr
		}
	}
	if uc.withdraws != nil {
		n, err := uc.withdraws.CountByStatuses(ctx, WithdrawRewarded, WithdrawApproved)
		if err != nil {
			return nil, err
		}
		view.QueueRewarded = n
		n2, err := uc.withdraws.CountByStatuses(ctx, WithdrawDoing)
		if err != nil {
			return nil, err
		}
		view.QueueDoing = n2
	}
	return view, nil
}
