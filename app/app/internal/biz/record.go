package biz

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

const (
	WithdrawPending   = "pending"
	WithdrawRewarded  = "rewarded" // 已审待打款（ADR 0010）
	WithdrawDoing     = "doing"    // 打款中
	WithdrawPass      = "pass"     // 链上成功
	WithdrawApproved  = "approved" // 历史兼容，视同 rewarded
	WithdrawRejected  = "rejected"
	WithdrawCancelled = "cancelled"
)

// Location is a subscription order (persisted as locations).
type Location struct {
	ID            uint64
	UserID        uint64
	Amount        decimal.Decimal
	Multiplier    decimal.Decimal
	ExitTarget    decimal.Decimal
	Accumulated   decimal.Decimal
	Status        string
	RatePercent   decimal.Decimal
	RateDirection string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// LocationRepo CRUD for 认购订单.
type LocationRepo interface {
	Create(ctx context.Context, loc *Location) (*Location, error)
	FindByID(ctx context.Context, id uint64) (*Location, error)
	Update(ctx context.Context, loc *Location) (*Location, error)
	ListByUser(ctx context.Context, userID uint64, status string) ([]*Location, error)
	ListActive(ctx context.Context, orderIDs []uint64) ([]*Location, error)
	FindEarliestActive(ctx context.Context, userID uint64) (*Location, error)
	SumAmountsByUser(ctx context.Context) (map[uint64]decimal.Decimal, error)
	ListAllPaged(ctx context.Context, address string, page, pageSize int) ([]*Location, int, error)
	SumActiveByUser(ctx context.Context, userID uint64) (decimal.Decimal, error)
	SumAmountByUser(ctx context.Context, userID uint64) (decimal.Decimal, error)
	SumAllAmount(ctx context.Context) (decimal.Decimal, error)
	SumAmountCreatedBetween(ctx context.Context, from, to time.Time) (decimal.Decimal, error)
	SumAmountByPathPrefix(ctx context.Context, pathPrefix string) (decimal.Decimal, error)
	CountDistinctUsers(ctx context.Context) (int, error)
	CountUsersFirstCreatedBetween(ctx context.Context, from, to time.Time) (int, error)
}

// Withdraw extract-asset application (withdraws table).
type Withdraw struct {
	ID             uint64
	UserID         uint64
	Amount         decimal.Decimal
	FeeAmount      decimal.Decimal
	CreditedAmount decimal.Decimal
	OrderIDs       []uint64
	Status         string
	Remark         string
	TxHash         string
	PayoutError    string
	ReviewedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// WithdrawRepo extract flow persistence.
type WithdrawRepo interface {
	Create(ctx context.Context, w *Withdraw) (*Withdraw, error)
	FindByID(ctx context.Context, id uint64) (*Withdraw, error)
	ListByUser(ctx context.Context, userID uint64, status string) ([]*Withdraw, error)
	ListFiltered(ctx context.Context, status string, userID uint64) ([]*Withdraw, error)
	ListPayoutQueue(ctx context.Context, limit int) ([]*Withdraw, error)
	CountByStatuses(ctx context.Context, statuses ...string) (int64, error)
	CasUpdateStatus(ctx context.Context, id uint64, fromStatus, toStatus, remark string) (bool, error)
	CasClaimPayout(ctx context.Context, id uint64) (bool, error)
	SetTxHash(ctx context.Context, id uint64, txHash string) error
	MarkPass(ctx context.Context, id uint64) (bool, error)
	SetPayoutError(ctx context.Context, id uint64, msg string) error
	SumApprovedAmount(ctx context.Context) (decimal.Decimal, error)
	SumApprovedAmountBetween(ctx context.Context, from, to time.Time) (decimal.Decimal, error)
}

// DashboardStats aggregates admin dashboard KPIs.
type DashboardStats struct {
	TotalUserR    int
	TotalUser     int
	TodayUserR    int
	TodayUser     int
	BuyTotal      decimal.Decimal
	TodayBuy      decimal.Decimal
	BalanceUsdt   decimal.Decimal
	TodayOne      decimal.Decimal
	TodayTwo      decimal.Decimal
	TodayThree    decimal.Decimal
	TotalReward   decimal.Decimal
	TodayWithdraw decimal.Decimal
	TotalWithdraw decimal.Decimal
	TotalIspay    decimal.Decimal
}

// SettleResult summarizes one static settle run.
type SettleResult struct {
	SettledCount    int
	ExitedCount     int
	GenerationCount int
	CommunityCount  int
	PeerCount       int
	Skipped         bool   // same-day already settled
	Forced          bool
	SettleDate      string // YYYY-MM-DD Asia/Shanghai
}

// RecordUseCase subscribe, withdraw, settle, ledger flows.
type RecordUseCase struct {
	locations        LocationRepo
	withdraws        WithdrawRepo
	ledger           LedgerRepo
	balances         UserBalanceRepo
	users            UserRepo
	recommends       RecommendRepo
	params           ParamsRepo
	ethUserRecord    EthUserRecordRepo
	settleRuns       SettleRunRepo
	dbPing           DBPinger
	allowForceSettle bool
	payoutCfg        PayoutConfig
	settleMu         sync.Mutex
}

// DBPinger probes database connectivity.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// NewRecordUseCase constructs RecordUseCase.
func NewRecordUseCase(
	locations LocationRepo,
	withdraws WithdrawRepo,
	ledger LedgerRepo,
	balances UserBalanceRepo,
	users UserRepo,
	recommends RecommendRepo,
	params ParamsRepo,
	ethUserRecord EthUserRecordRepo,
	settleRuns SettleRunRepo,
	allowForceSettle bool,
) *RecordUseCase {
	return &RecordUseCase{
		locations:        locations,
		withdraws:        withdraws,
		ledger:           ledger,
		balances:         balances,
		users:            users,
		recommends:       recommends,
		params:           params,
		ethUserRecord:    ethUserRecord,
		settleRuns:       settleRuns,
		allowForceSettle: allowForceSettle,
	}
}

// SetDBPinger wires readiness ping (optional; called from composition root).
func (uc *RecordUseCase) SetDBPinger(p DBPinger) {
	uc.dbPing = p
}

// PingDB returns nil when MySQL is reachable.
func (uc *RecordUseCase) PingDB(ctx context.Context) error {
	if uc.dbPing == nil {
		return nil
	}
	return uc.dbPing.Ping(ctx)
}

// SettleStatusView is admin-facing last/today settle snapshot.
type SettleStatusView struct {
	TodayDate       string
	TodaySettled    bool
	Today           *SettleRun
	Latest          *SettleRun
	AllowForce      bool
}

// SettleStatus returns today + latest settle_runs for ops.
func (uc *RecordUseCase) SettleStatus(ctx context.Context) (*SettleStatusView, error) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	now := time.Now().In(loc)
	settleDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	dateStr := settleDay.Format("2006-01-02")

	view := &SettleStatusView{
		TodayDate:  dateStr,
		AllowForce: uc.allowForceSettle,
	}
	if uc.settleRuns == nil {
		return view, nil
	}
	today, err := uc.settleRuns.FindByDate(ctx, settleDay)
	if err != nil {
		return nil, err
	}
	view.Today = today
	view.TodaySettled = today != nil
	latest, err := uc.settleRuns.FindLatest(ctx)
	if err != nil {
		return nil, err
	}
	view.Latest = latest
	return view, nil
}

// CreateLocation opens a subscription order and deducts account balance.
func (uc *RecordUseCase) CreateLocation(ctx context.Context, userID uint64, amount decimal.Decimal) (*Location, error) {
	if !amount.IsPositive() {
		return nil, ErrInvalidAmount
	}
	minSub, err := decimal.NewFromString(GetActiveParams().MinSubscribeAmount)
	if err != nil || !minSub.IsPositive() {
		minSub = decimal.NewFromInt(100)
	}
	if amount.LessThan(minSub) {
		return nil, ErrInvalidAmount
	}
	multiplier, ok := MultiplierForAmount(amount)
	if !ok {
		return nil, ErrInvalidAmount
	}
	user, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if user.AccountBalance.LessThan(amount) {
		return nil, ErrInsufficientBalance
	}
	if err := uc.balances.SubAccountBalance(ctx, userID, amount); err != nil {
		return nil, err
	}
	rate, direction := InitialRate()
	loc := &Location{
		UserID:        userID,
		Amount:        amount,
		Multiplier:    multiplier,
		ExitTarget:    amount.Mul(multiplier),
		Accumulated:   decimal.Zero,
		Status:        OrderStatusActive,
		RatePercent:   rate,
		RateDirection: direction,
	}
	return uc.locations.Create(ctx, loc)
}

// ListLocations lists orders for a user.
func (uc *RecordUseCase) ListLocations(ctx context.Context, userID uint64, status string) ([]*Location, error) {
	return uc.locations.ListByUser(ctx, userID, status)
}

// CreateWithdraw submits an extract request.
// orderIDs are ignored: rate turnaround applies to all active locations at approve time.
func (uc *RecordUseCase) CreateWithdraw(ctx context.Context, userID uint64, amount decimal.Decimal, _ []uint64) (*Withdraw, error) {
	if !amount.IsPositive() {
		return nil, ErrInvalidAmount
	}
	minWd, err := decimal.NewFromString(GetActiveParams().MinWithdrawAmount)
	if err != nil || !minWd.IsPositive() {
		minWd = decimal.NewFromInt(10)
	}
	if amount.LessThan(minWd) {
		return nil, ErrBelowMinWithdraw
	}
	user, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if user.WithdrawableBalance.LessThan(amount) {
		return nil, ErrInsufficientBalance
	}

	feeRate := decimal.RequireFromString(GetActiveParams().ExtractFeeRate)
	fee := amount.Mul(feeRate).Round(8)
	credited := amount.Sub(fee)

	if err := uc.balances.SubWithdrawableBalance(ctx, userID, amount); err != nil {
		return nil, err
	}

	created, err := uc.withdraws.Create(ctx, &Withdraw{
		UserID:         userID,
		Amount:         amount,
		FeeAmount:      fee,
		CreditedAmount: credited,
		OrderIDs:       nil,
		Status:         WithdrawPending,
	})
	if err != nil {
		_ = uc.balances.AddWithdrawableBalance(ctx, userID, amount)
		return nil, err
	}
	return created, nil
}

// CancelWithdraw cancels a pending request and refunds withdrawable.
func (uc *RecordUseCase) CancelWithdraw(ctx context.Context, userID, id uint64) (*Withdraw, error) {
	w, err := uc.withdraws.FindByID(ctx, id)
	if err != nil || w == nil || w.UserID != userID {
		return nil, ErrWithdrawNotFound
	}
	ok, err := uc.withdraws.CasUpdateStatus(ctx, id, WithdrawPending, WithdrawCancelled, "")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrWithdrawConflict
	}
	if err := uc.balances.AddWithdrawableBalance(ctx, userID, w.Amount); err != nil {
		return nil, err
	}
	w.Status = WithdrawCancelled
	return w, nil
}

// ApproveWithdraw confirms pending request → rewarded (awaiting on-chain payout), turns rates.
func (uc *RecordUseCase) ApproveWithdraw(ctx context.Context, id uint64) (*Withdraw, error) {
	w, err := uc.withdraws.FindByID(ctx, id)
	if err != nil || w == nil {
		return nil, ErrWithdrawNotFound
	}
	ok, err := uc.withdraws.CasUpdateStatus(ctx, id, WithdrawPending, WithdrawRewarded, "")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrWithdrawConflict
	}
	locs, err := uc.locations.ListByUser(ctx, w.UserID, OrderStatusActive)
	if err != nil {
		return nil, err
	}
	for _, loc := range locs {
		loc.RatePercent, loc.RateDirection = ApplyExtractRateTurn(loc.RatePercent, loc.RateDirection)
		if _, err := uc.locations.Update(ctx, loc); err != nil {
			return nil, err
		}
	}
	_ = uc.writeLedger(ctx, w.UserID, nil, LedgerExtract, w.Amount.Neg(), "withdraw approved fee="+w.FeeAmount.String())
	w.Status = WithdrawRewarded
	now := time.Now()
	w.ReviewedAt = &now
	return w, nil
}

// RejectWithdraw rejects pending request and refunds withdrawable.
func (uc *RecordUseCase) RejectWithdraw(ctx context.Context, id uint64, remark string) (*Withdraw, error) {
	w, err := uc.withdraws.FindByID(ctx, id)
	if err != nil || w == nil {
		return nil, ErrWithdrawNotFound
	}
	ok, err := uc.withdraws.CasUpdateStatus(ctx, id, WithdrawPending, WithdrawRejected, remark)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrWithdrawConflict
	}
	if err := uc.balances.AddWithdrawableBalance(ctx, w.UserID, w.Amount); err != nil {
		return nil, err
	}
	w.Status = WithdrawRejected
	w.Remark = remark
	now := time.Now()
	w.ReviewedAt = &now
	return w, nil
}

// ListWithdraws lists user withdraws.
func (uc *RecordUseCase) ListWithdraws(ctx context.Context, userID uint64, status string) ([]*Withdraw, error) {
	return uc.withdraws.ListByUser(ctx, userID, status)
}

// ListWithdrawsAdmin lists withdraws for admin.
func (uc *RecordUseCase) ListWithdrawsAdmin(ctx context.Context, status string, userID uint64) ([]*Withdraw, error) {
	return uc.withdraws.ListFiltered(ctx, status, userID)
}

// ListLocationsAdmin lists locations for admin with pagination.
func (uc *RecordUseCase) ListLocationsAdmin(ctx context.Context, address string, page, pageSize int) ([]*Location, int, error) {
	return uc.locations.ListAllPaged(ctx, address, page, pageSize)
}

// ListLedgerAdmin lists ledger entries for admin with pagination.
func (uc *RecordUseCase) ListLedgerAdmin(ctx context.Context, address, entryType string, page, pageSize int) ([]*LedgerEntry, int, error) {
	return uc.ledger.ListPaged(ctx, address, entryType, page, pageSize)
}

// ListUsersAdmin lists users for admin with pagination.
func (uc *RecordUseCase) ListUsersAdmin(ctx context.Context, address string, page, pageSize int) ([]*User, int, error) {
	return uc.users.ListPaged(ctx, address, page, pageSize)
}

// ListDirectReferrals returns direct referrals for a wallet address.
func (uc *RecordUseCase) ListDirectReferrals(ctx context.Context, address string) ([]*User, error) {
	user, err := uc.users.FindByAddress(ctx, address)
	if err != nil {
		return nil, err
	}
	return uc.users.ListByInviter(ctx, user.ID)
}

// ListDirectReferralsByInviter returns direct referrals for an inviter user id.
func (uc *RecordUseCase) ListDirectReferralsByInviter(ctx context.Context, inviterID uint64) ([]*User, error) {
	return uc.users.ListByInviter(ctx, inviterID)
}

// GetUserByID loads a user by id.
func (uc *RecordUseCase) GetUserByID(ctx context.Context, id uint64) (*User, error) {
	return uc.users.FindByID(ctx, id)
}

// SumActiveAmount sums active location amounts for a user.
func (uc *RecordUseCase) SumActiveAmount(ctx context.Context, userID uint64) (decimal.Decimal, error) {
	return uc.locations.SumActiveByUser(ctx, userID)
}

// SumLocationAmount sums all location amounts for a user (active + exited).
func (uc *RecordUseCase) SumLocationAmount(ctx context.Context, userID uint64) (decimal.Decimal, error) {
	return uc.locations.SumAmountByUser(ctx, userID)
}

// CountDirectReferrals counts direct referrals for a user.
func (uc *RecordUseCase) CountDirectReferrals(ctx context.Context, userID uint64) (int, error) {
	return uc.users.CountDirectReferrals(ctx, userID)
}

// ListAllUsers returns all users (admin dashboard).
func (uc *RecordUseCase) ListAllUsers(ctx context.Context) ([]*User, error) {
	return uc.users.ListAll(ctx)
}

// ListLedger lists ledger entries for a user in a time range.
func (uc *RecordUseCase) ListLedger(ctx context.Context, userID uint64, from, to time.Time) ([]*LedgerEntry, error) {
	return uc.ledger.ListByUser(ctx, userID, from, to)
}

// ListConfigs lists business config rows.
func (uc *RecordUseCase) ListConfigs(ctx context.Context) ([]*BusinessConfig, error) {
	return uc.params.ListConfigs(ctx)
}

// UpdateConfig updates one config value and hot-reloads params.
func (uc *RecordUseCase) UpdateConfig(ctx context.Context, id uint64, value string) (*BusinessConfig, error) {
	cfg, err := uc.params.UpdateConfigValue(ctx, id, value)
	if err != nil {
		return nil, err
	}
	if p, err := uc.params.Get(ctx); err == nil {
		SetActiveParams(p)
	}
	return cfg, nil
}

// SettleStatic runs static → generation → refresh V → community base → peer.
// Same calendar day (Asia/Shanghai): TryClaim INSERT occupies the day (multi-instance safe);
// conflict → skipped. force=true only when allowForceSettle (may double-credit; test only).
// If claim succeeds but process crashes mid-settle, the day stays claimed and will not auto-rerun (R1).
func (uc *RecordUseCase) SettleStatic(ctx context.Context, orderIDs []uint64, force bool) (*SettleResult, error) {
	uc.settleMu.Lock()
	defer uc.settleMu.Unlock()

	if force && !uc.allowForceSettle {
		return nil, ErrForceSettleDisabled
	}

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	now := time.Now().In(loc)
	settleDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	dateStr := settleDay.Format("2006-01-02")

	if uc.settleRuns != nil && !force {
		claimed, err := uc.settleRuns.TryClaim(ctx, settleDay, "settle claim")
		if err != nil {
			return nil, err
		}
		if !claimed {
			return &SettleResult{
				Skipped:    true,
				SettleDate: dateStr,
			}, nil
		}
	}

	result, err := uc.settleStaticOnce(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	result.Forced = force
	result.SettleDate = dateStr

	if uc.settleRuns != nil {
		remark := "settle"
		if force {
			remark = "settle force=1 (may double-credit)"
		}
		_ = uc.settleRuns.Upsert(ctx, &SettleRun{
			SettleDate:      settleDay,
			Forced:          force,
			SettledCount:    result.SettledCount,
			ExitedCount:     result.ExitedCount,
			GenerationCount: result.GenerationCount,
			CommunityCount:  result.CommunityCount,
			PeerCount:       result.PeerCount,
			Remark:          remark,
		})
	}
	return result, nil
}

func (uc *RecordUseCase) settleStaticOnce(ctx context.Context, orderIDs []uint64) (*SettleResult, error) {
	orders, err := uc.locations.ListActive(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	result := &SettleResult{}
	todayStatic := map[uint64]decimal.Decimal{}
	for _, snap := range orders {
		// Re-load: earlier orders' 代数 may have filled/exited this row already.
		order, err := uc.locations.FindByID(ctx, snap.ID)
		if err != nil {
			if errors.Is(err, ErrLocationNotFound) {
				continue
			}
			return nil, err
		}
		if order.Status != OrderStatusActive {
			continue
		}
		room := order.ExitTarget.Sub(order.Accumulated)
		if !room.IsPositive() {
			order.Status = OrderStatusExited
			if _, err := uc.locations.Update(ctx, order); err != nil {
				return nil, err
			}
			result.ExitedCount++
			continue
		}
		yield := CalcStaticYield(order.ExitTarget, order.RatePercent)
		if !yield.IsPositive() {
			continue
		}
		if yield.GreaterThan(room) {
			yield = room
		}
		order.Accumulated = order.Accumulated.Add(yield)
		if err := uc.balances.AddWithdrawableBalance(ctx, order.UserID, yield); err != nil {
			return nil, err
		}
		oid := order.ID
		if err := uc.writeLedger(ctx, order.UserID, &oid, LedgerStatic, yield, "static yield"); err != nil {
			return nil, err
		}

		if order.Accumulated.GreaterThanOrEqual(order.ExitTarget) {
			order.Accumulated = order.ExitTarget
			order.Status = OrderStatusExited
			result.ExitedCount++
		} else {
			order.RatePercent, order.RateDirection = AdvanceRate(order.RatePercent, order.RateDirection)
		}
		if _, err := uc.locations.Update(ctx, order); err != nil {
			return nil, err
		}
		result.SettledCount++
		todayStatic[order.UserID] = todayStatic[order.UserID].Add(yield)

		n, err := uc.payGenerationRewards(ctx, order.UserID, yield)
		if err != nil {
			return nil, err
		}
		result.GenerationCount += n
	}

	contrib := map[uint64]decimal.Decimal{}
	for uid, y := range todayStatic {
		u, err := uc.users.FindByID(ctx, uid)
		if err != nil {
			continue
		}
		if u.RewardLocked {
			continue
		}
		contrib[uid] = y
	}

	cc, pc, err := uc.settleCommunity(ctx, contrib)
	if err != nil {
		return nil, err
	}
	result.CommunityCount = cc
	result.PeerCount = pc
	return result, nil
}

func (uc *RecordUseCase) settleCommunity(ctx context.Context, todayStatic map[uint64]decimal.Decimal) (communityPaid, peerPaid int, err error) {
	users, err := uc.users.ListAll(ctx)
	if err != nil {
		return 0, 0, err
	}
	personal, err := uc.locations.SumAmountsByUser(ctx)
	if err != nil {
		return 0, 0, err
	}
	if personal == nil {
		personal = map[uint64]decimal.Decimal{}
	}

	children := map[uint64][]uint64{}
	parent := map[uint64]uint64{}
	for _, u := range users {
		if u.InviterID != nil {
			pid := *u.InviterID
			parent[u.ID] = pid
			children[pid] = append(children[pid], u.ID)
		}
		if _, ok := personal[u.ID]; !ok {
			personal[u.ID] = decimal.Zero
		}
	}

	levels := map[uint64]int{}
	for _, u := range users {
		legs := children[u.ID]
		legVols := make([]decimal.Decimal, len(legs))
		for i, leg := range legs {
			legVols[i] = SubtreeVolume(leg, children, personal)
		}
		vol := SmallAreaVolume(legVols)
		lv := LevelFromVolume(vol)
		levels[u.ID] = lv
		if err := uc.users.UpdateCommunity(ctx, u.ID, uint8(lv), vol); err != nil {
			return 0, 0, err
		}
	}

	for _, u := range users {
		base := CalcCommunityBase(u.ID, children, parent, levels, personal, todayStatic)
		if !base.IsPositive() {
			continue
		}
		applied, err := uc.creditOrderAcceleration(ctx, u.ID, base, LedgerCommunityBase, "community base")
		if err != nil {
			return communityPaid, peerPaid, err
		}
		if applied.IsPositive() {
			communityPaid++
		}
	}

	peers := CalcPeerRewards(children, parent, levels, personal, todayStatic)
	for uid, amount := range peers {
		if !amount.IsPositive() {
			continue
		}
		applied, err := uc.creditOrderAcceleration(ctx, uid, amount, LedgerPeer, "peer reward")
		if err != nil {
			return communityPaid, peerPaid, err
		}
		if applied.IsPositive() {
			peerPaid++
		}
	}
	return communityPaid, peerPaid, nil
}

func (uc *RecordUseCase) payGenerationRewards(ctx context.Context, fromUserID uint64, staticYield decimal.Decimal) (int, error) {
	reward := CalcGenerationReward(staticYield)
	if !reward.IsPositive() {
		return 0, nil
	}
	user, err := uc.users.FindByID(ctx, fromUserID)
	if err != nil {
		return 0, err
	}
	if user.RewardLocked {
		return 0, nil
	}
	paid := 0
	current := user
	for depth := 1; depth <= maxGenerationDepth(); depth++ {
		if current.InviterID == nil {
			break
		}
		inviterID := *current.InviterID
		inviter, err := uc.users.FindByID(ctx, inviterID)
		if err != nil {
			return paid, err
		}
		directs, err := uc.users.CountDirectReferrals(ctx, inviterID)
		if err != nil {
			return paid, err
		}
		if CanEarnGeneration(directs, depth) {
			remark := fmt.Sprintf("generation from=%s depth=%d", user.Address, depth)
			applied, err := uc.creditOrderAcceleration(ctx, inviterID, reward, LedgerGeneration, remark)
			if err != nil {
				return paid, err
			}
			if applied.IsPositive() {
				paid++
			}
		}
		current = inviter
	}
	return paid, nil
}

func (uc *RecordUseCase) creditDailyDividend(ctx context.Context, userID uint64, amount decimal.Decimal, entryType, remark string) error {
	if !amount.IsPositive() {
		return nil
	}
	if err := uc.balances.AddWithdrawableBalance(ctx, userID, amount); err != nil {
		return err
	}
	var orderID *uint64
	order, err := uc.locations.FindEarliestActive(ctx, userID)
	if err != nil {
		if !errors.Is(err, ErrLocationNotFound) {
			return err
		}
	} else {
		order.Accumulated = order.Accumulated.Add(amount)
		if order.Accumulated.GreaterThanOrEqual(order.ExitTarget) {
			order.Accumulated = order.ExitTarget
			order.Status = OrderStatusExited
		}
		if _, err := uc.locations.Update(ctx, order); err != nil {
			return err
		}
		oid := order.ID
		orderID = &oid
	}
	return uc.writeLedger(ctx, userID, orderID, entryType, amount, remark)
}

// creditOrderAcceleration applies 代数/社区基础奖/平级：FIFO 填进行中认购单（进度+可提双记）。
// 无进行中订单则整笔作废；填满全部进行中单后的剩余亦作废。返回实际入账金额。
func (uc *RecordUseCase) creditOrderAcceleration(ctx context.Context, userID uint64, amount decimal.Decimal, entryType, remark string) (decimal.Decimal, error) {
	if !amount.IsPositive() {
		return decimal.Zero, nil
	}
	remaining := amount
	applied := decimal.Zero
	var lastOrderID *uint64
	for remaining.IsPositive() {
		order, err := uc.locations.FindEarliestActive(ctx, userID)
		if err != nil {
			if errors.Is(err, ErrLocationNotFound) {
				break
			}
			return decimal.Zero, err
		}
		chunk, room := chunkAcceleration(order.Accumulated, order.ExitTarget, remaining)
		if !room.IsPositive() {
			order.Status = OrderStatusExited
			if _, err := uc.locations.Update(ctx, order); err != nil {
				return decimal.Zero, err
			}
			continue
		}
		order.Accumulated = order.Accumulated.Add(chunk)
		if order.Accumulated.GreaterThanOrEqual(order.ExitTarget) {
			order.Accumulated = order.ExitTarget
			order.Status = OrderStatusExited
		}
		if _, err := uc.locations.Update(ctx, order); err != nil {
			return decimal.Zero, err
		}
		oid := order.ID
		lastOrderID = &oid
		applied = applied.Add(chunk)
		remaining = remaining.Sub(chunk)
	}
	if !applied.IsPositive() {
		return decimal.Zero, nil
	}
	if err := uc.balances.AddWithdrawableBalance(ctx, userID, applied); err != nil {
		return decimal.Zero, err
	}
	finalRemark := remark
	if remaining.IsPositive() {
		finalRemark = fmt.Sprintf("%s applied=%s discarded=%s", remark, applied.String(), remaining.String())
	}
	if err := uc.writeLedger(ctx, userID, lastOrderID, entryType, applied, finalRemark); err != nil {
		return decimal.Zero, err
	}
	return applied, nil
}

// chunkAcceleration returns how much of amount fits into one order's remaining exit room.
func chunkAcceleration(accumulated, exitTarget, amount decimal.Decimal) (chunk, room decimal.Decimal) {
	room = exitTarget.Sub(accumulated)
	if !room.IsPositive() {
		return decimal.Zero, decimal.Zero
	}
	chunk = amount
	if chunk.GreaterThan(room) {
		chunk = room
	}
	return chunk, room
}

// SetVIP sets community level (V0–V9) without changing volume.
func (uc *RecordUseCase) SetVIP(ctx context.Context, userID uint64, level int) error {
	if level < 0 || level > 9 {
		return ErrInvalidAmount
	}
	if _, err := uc.users.FindByID(ctx, userID); err != nil {
		return err
	}
	return uc.users.SetCommunityLevel(ctx, userID, uint8(level))
}

// SetUserLock soft-deletes or restores a single user.
func (uc *RecordUseCase) SetUserLock(ctx context.Context, userID uint64, lock bool) error {
	if _, err := uc.users.FindByID(ctx, userID); err != nil {
		return err
	}
	if lock {
		return uc.users.SoftDelete(ctx, userID, time.Now())
	}
	return uc.users.Restore(ctx, userID)
}

// SetLineLock locks or unlocks a user and all descendants in the recommend tree.
func (uc *RecordUseCase) SetLineLock(ctx context.Context, userID uint64, lock bool) error {
	path, err := uc.recommends.GetPath(ctx, userID)
	if err != nil {
		return err
	}
	if path == "" {
		path = formatUserPath(userID)
	}
	users, err := uc.users.ListByPathPrefix(ctx, path)
	if err != nil {
		return err
	}
	for _, u := range users {
		if lock {
			if err := uc.users.SoftDelete(ctx, u.ID, time.Now()); err != nil {
				return err
			}
		} else if err := uc.users.Restore(ctx, u.ID); err != nil {
			return err
		}
	}
	return nil
}

// SetRewardLock toggles 停分红 (reward_locked).
func (uc *RecordUseCase) SetRewardLock(ctx context.Context, userID uint64, locked bool) error {
	if _, err := uc.users.FindByID(ctx, userID); err != nil {
		return err
	}
	return uc.users.SetRewardLocked(ctx, userID, locked)
}

// DashboardStats computes admin dashboard KPIs for the given instant (day bounds use Asia/Shanghai).
func (uc *RecordUseCase) DashboardStats(ctx context.Context, now time.Time) (*DashboardStats, error) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	local := now.In(loc)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	end := start.Add(24 * time.Hour)

	totalUserR, err := uc.users.CountAll(ctx)
	if err != nil {
		return nil, err
	}
	totalUser, err := uc.locations.CountDistinctUsers(ctx)
	if err != nil {
		return nil, err
	}
	todayUserR, err := uc.users.CountCreatedBetween(ctx, start, end)
	if err != nil {
		return nil, err
	}
	todayUser, err := uc.locations.CountUsersFirstCreatedBetween(ctx, start, end)
	if err != nil {
		return nil, err
	}

	buyTotal, err := uc.locations.SumAllAmount(ctx)
	if err != nil {
		return nil, err
	}
	todayBuy, err := uc.locations.SumAmountCreatedBetween(ctx, start, end)
	if err != nil {
		return nil, err
	}
	balanceUsdt, err := uc.users.SumWithdrawableBalance(ctx)
	if err != nil {
		return nil, err
	}
	totalIspay, err := uc.users.SumAccountBalance(ctx)
	if err != nil {
		return nil, err
	}

	staticTypes := []string{LedgerStatic}
	dynamicTypes := []string{LedgerGeneration, LedgerCommunityBase, LedgerPeer}
	rewardTypes := append(append([]string{}, staticTypes...), dynamicTypes...)

	todayOne, err := uc.ledger.SumAmountByTypesBetween(ctx, staticTypes, start, end)
	if err != nil {
		return nil, err
	}
	todayTwo, err := uc.ledger.SumAmountByTypesBetween(ctx, dynamicTypes, start, end)
	if err != nil {
		return nil, err
	}
	totalReward, err := uc.ledger.SumAmountByTypes(ctx, rewardTypes)
	if err != nil {
		return nil, err
	}
	totalWithdraw, err := uc.withdraws.SumApprovedAmount(ctx)
	if err != nil {
		return nil, err
	}
	todayWithdraw, err := uc.withdraws.SumApprovedAmountBetween(ctx, start, end)
	if err != nil {
		return nil, err
	}

	return &DashboardStats{
		TotalUserR:    totalUserR,
		TotalUser:     totalUser,
		TodayUserR:    todayUserR,
		TodayUser:     todayUser,
		BuyTotal:      buyTotal,
		TodayBuy:      todayBuy,
		BalanceUsdt:   balanceUsdt,
		TodayOne:      todayOne,
		TodayTwo:      todayTwo,
		TodayThree:    todayOne.Add(todayTwo),
		TotalReward:   totalReward,
		TodayWithdraw: todayWithdraw,
		TotalWithdraw: totalWithdraw,
		TotalIspay:    totalIspay,
	}, nil
}

// SumSubtreeLocationAmount sums all location amounts for a user and their descendants.
func (uc *RecordUseCase) SumSubtreeLocationAmount(ctx context.Context, userID uint64) (decimal.Decimal, error) {
	path, err := uc.recommends.GetPath(ctx, userID)
	if err != nil {
		return decimal.Zero, err
	}
	if path == "" {
		path = formatUserPath(userID)
	}
	return uc.locations.SumAmountByPathPrefix(ctx, path)
}

// AreaVolumes returns team total / max-leg (大区) / small-area (小区) volumes for a user.
// Volumes are historical subscription amounts of downlines only (excludes self).
func (uc *RecordUseCase) AreaVolumes(ctx context.Context, userID uint64) (total, maxLeg, smallArea decimal.Decimal, err error) {
	directs, err := uc.ListDirectReferralsByInviter(ctx, userID)
	if err != nil {
		return decimal.Zero, decimal.Zero, decimal.Zero, err
	}
	legVols := make([]decimal.Decimal, 0, len(directs))
	for _, d := range directs {
		vol, err := uc.SumSubtreeLocationAmount(ctx, d.ID)
		if err != nil {
			return decimal.Zero, decimal.Zero, decimal.Zero, err
		}
		legVols = append(legVols, vol)
		total = total.Add(vol)
	}
	if len(legVols) == 0 {
		return decimal.Zero, decimal.Zero, decimal.Zero, nil
	}
	idx := MaxLegIndex(legVols)
	if idx >= 0 {
		maxLeg = legVols[idx]
	}
	smallArea = SmallAreaVolume(legVols)
	return total, maxLeg, smallArea, nil
}

func formatUserPath(userID uint64) string {
	return strconv.FormatUint(userID, 10)
}

func (uc *RecordUseCase) writeLedger(ctx context.Context, userID uint64, orderID *uint64, entryType string, amount decimal.Decimal, remark string) error {
	if uc.ledger == nil {
		return nil
	}
	return uc.ledger.Create(ctx, &LedgerEntry{
		UserID:      userID,
		OrderID:     orderID,
		EntryType:   entryType,
		Amount:      amount,
		BalanceKind: BalanceWithdrawable,
		Remark:      remark,
	})
}

// GetEthUserRecordLast 对齐 new18new；空表返回 -1。
func (uc *RecordUseCase) GetEthUserRecordLast(ctx context.Context) (int64, error) {
	if uc.ethUserRecord == nil {
		return -1, nil
	}
	return uc.ethUserRecord.GetEthUserRecordLast(ctx)
}

const (
	ChainDepositStatusSuccess = "success"
	ChainDepositStatusSkipped = "skipped"
	ChainDepositSkipUnreg     = "unregistered"
	ChainDepositSkipBelowMin  = "below_min"
	MinChainDepositAmount     = 100
)

// ClaimChainDepositSkip writes a skipped row for contract index (advances cursor).
func (uc *RecordUseCase) ClaimChainDepositSkip(ctx context.Context, index int64, amount int64, address, reason string) error {
	if uc.ethUserRecord == nil {
		return nil
	}
	_, err := uc.ethUserRecord.CreateEthUserRecordListByHash(ctx, &EthUserRecord{
		UserId:    0,
		Hash:      strings.ToLower(address),
		Status:    ChainDepositStatusSkipped,
		Type:      reason,
		Amount:    strconv.FormatInt(amount, 10),
		AmountTwo: uint64(max64(amount, 0)),
		CoinType:  "USDT",
		Last:      index,
	})
	return err
}

// CreditChainDeposit claims index then credits account balance (ADR 0007 C2).
func (uc *RecordUseCase) CreditChainDeposit(ctx context.Context, userID uint64, index int64, amount int64, address string) error {
	if amount < MinChainDepositAmount || userID == 0 {
		return ErrInvalidAmount
	}
	if uc.ethUserRecord == nil {
		return ErrNotImplemented
	}
	amt := decimal.NewFromInt(amount)
	_, err := uc.ethUserRecord.CreateEthUserRecordListByHash(ctx, &EthUserRecord{
		UserId:    int64(userID),
		Hash:      strings.ToLower(address),
		Status:    ChainDepositStatusSuccess,
		Type:      "deposit",
		Amount:    strconv.FormatInt(amount, 10) + "000000000000000000",
		AmountTwo: uint64(amount),
		CoinType:  "USDT",
		Last:      index,
	})
	if err != nil {
		return err
	}
	if err := uc.balances.AddAccountBalance(ctx, userID, amt); err != nil {
		_ = uc.ethUserRecord.DeleteByLast(ctx, index)
		return err
	}
	if uc.ledger != nil {
		if err := uc.ledger.Create(ctx, &LedgerEntry{
			UserID:      userID,
			EntryType:   LedgerDeposit,
			Amount:      amt,
			BalanceKind: BalanceAccount,
			Remark:      "chain deposit",
		}); err != nil {
			// balance already moved; keep eth row so cursor does not re-credit
			fmt.Println("chain deposit ledger:", err)
		}
	}
	return nil
}

const (
	ReplayStatusCredited        = "credited"
	ReplayStatusAlreadyCredited = "already_credited"
	ReplayStatusStillSkipped    = "still_skipped"
	ReplayStatusNotFound        = "not_found"
)

// ReplayChainDepositResult is the admin replay outcome for one BuySomething index.
type ReplayChainDepositResult struct {
	Status             string
	Index              int64
	Address            string
	Amount             int64
	UserID             uint64
	PreviousStatus     string
	PreviousSkipReason string
	SkipReason         string
}

// ReplaySkippedChainDeposit turns a skipped eth_user_record into a credited deposit.
// amount/address must come from chain (caller responsibility). Only status=skipped rows are eligible.
func (uc *RecordUseCase) ReplaySkippedChainDeposit(ctx context.Context, index, amount int64, address string) (*ReplayChainDepositResult, error) {
	address = strings.ToLower(strings.TrimSpace(address))
	out := &ReplayChainDepositResult{
		Index:   index,
		Address: address,
		Amount:  amount,
	}
	if uc.ethUserRecord == nil {
		return nil, ErrNotImplemented
	}
	row, err := uc.ethUserRecord.GetByLast(ctx, index)
	if err != nil {
		return nil, err
	}
	if row == nil {
		out.Status = ReplayStatusNotFound
		return out, nil
	}
	out.PreviousStatus = row.Status
	out.PreviousSkipReason = row.Type
	if row.Status == ChainDepositStatusSuccess {
		out.Status = ReplayStatusAlreadyCredited
		out.UserID = uint64(row.UserId)
		return out, nil
	}
	if row.Status != ChainDepositStatusSkipped {
		return nil, ErrChainDepositNotSkipped
	}
	if amount < MinChainDepositAmount {
		out.Status = ReplayStatusStillSkipped
		out.SkipReason = ChainDepositSkipBelowMin
		return out, nil
	}
	users, err := uc.GetUserByAddress(ctx, address)
	if err != nil {
		return nil, err
	}
	u := users[address]
	if u == nil {
		out.Status = ReplayStatusStillSkipped
		out.SkipReason = ChainDepositSkipUnreg
		return out, nil
	}
	if err := uc.ethUserRecord.DeleteByLast(ctx, index); err != nil {
		return nil, err
	}
	if err := uc.CreditChainDeposit(ctx, u.ID, index, amount, address); err != nil {
		if errors.Is(err, ErrChainDepositExists) {
			out.Status = ReplayStatusAlreadyCredited
			out.UserID = u.ID
			return out, nil
		}
		// best-effort restore skipped marker so ops can retry
		_ = uc.ClaimChainDepositSkip(ctx, index, amount, address, row.Type)
		return nil, err
	}
	out.Status = ReplayStatusCredited
	out.UserID = u.ID
	return out, nil
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// DepositNew 链上充值入账户余额（保留；新拉单优先 CreditChainDeposit）.
func (uc *RecordUseCase) DepositNew(ctx context.Context, userId int64, pId int64, amount uint64, eth *EthUserRecord, system bool) error {
	_ = pId
	if amount == 0 || userId <= 0 {
		return ErrInvalidAmount
	}
	uid := uint64(userId)
	amt := decimal.NewFromInt(int64(amount))
	if err := uc.balances.AddAccountBalance(ctx, uid, amt); err != nil {
		return err
	}
	if uc.ledger != nil {
		_ = uc.ledger.Create(ctx, &LedgerEntry{
			UserID:      uid,
			EntryType:   LedgerDeposit,
			Amount:      amt,
			BalanceKind: BalanceAccount,
			Remark:      "chain deposit",
		})
	}
	if !system && uc.ethUserRecord != nil && eth != nil {
		eth.AmountTwo = amount
		if eth.Amount == "" {
			eth.Amount = strconv.FormatUint(amount, 10) + "000000000000000000"
		}
		if _, err := uc.ethUserRecord.CreateEthUserRecordListByHash(ctx, eth); err != nil {
			fmt.Println(err, "CREATE_ETH_USER_RECORD", userId, amount)
			return err
		}
	}
	return nil
}

// GetUserByAddress 批量按地址查用户（小写键）.
func (uc *RecordUseCase) GetUserByAddress(ctx context.Context, addresses ...string) (map[string]*User, error) {
	out := make(map[string]*User, len(addresses))
	for _, addr := range addresses {
		u, err := uc.users.FindByAddress(ctx, strings.ToLower(addr))
		if err != nil || u == nil {
			// try original casing if DB stored mixed
			u2, err2 := uc.users.FindByAddress(ctx, addr)
			if err2 != nil || u2 == nil {
				continue
			}
			u = u2
		}
		out[strings.ToLower(u.Address)] = u
	}
	return out, nil
}

