package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jinniu/app/app/app/internal/biz"
)

// 对齐 new18new：主网充值合约 BuySomething
const chainDepositContract = "0x49c735D94e1cc44053D23c956972cB37da3Fd5Af"

const (
	chainDepositMinAmount = biz.MinChainDepositAmount
	chainDepositChunk     = 50
	chainDepositRoundWall = 90 * time.Second
	chainDepositCatchupWall = 5 * time.Minute
	chainDepositCatchupMax  = 500
)

type userDeposit struct {
	Index   int64
	Address string
	Amount  int64
	Id      int64
}

type chainDepositOpts struct {
	Chunk      int
	RoundWall  time.Duration
	MaxIndices int // 0 = no index cap (time only)
}

// CompatAdminDepositChain GET 拉链上充值（ADR 0007 C2/C2+）.
// Query: until_caught_up=1 → multi-round until caught up or 5min/500 cap.
func (a *AppService) CompatAdminDepositChain(w http.ResponseWriter, r *http.Request) {
	untilCaughtUp := r.URL.Query().Get("until_caught_up") == "1" ||
		r.URL.Query().Get("until_caught_up") == "true"

	httpBudget := 120 * time.Second
	if untilCaughtUp {
		httpBudget = chainDepositCatchupWall + 30*time.Second
	}
	syncCtx, cancel := context.WithTimeout(context.Background(), httpBudget)
	defer cancel()

	var (
		res *chainDepositSyncResult
		err error
	)
	if untilCaughtUp {
		res, err = a.syncChainDepositsCatchUp(syncCtx)
	} else {
		res, err = a.syncChainDeposits(syncCtx, chainDepositOpts{
			Chunk:     chainDepositChunk,
			RoundWall: chainDepositRoundWall,
		})
	}
	if err != nil {
		writeBizError(w, err)
		return
	}
	out := map[string]any{
		"status":          "ok",
		"pulled":          res.Pulled,
		"credited":        res.Credited,
		"skipped":         res.Skipped,
		"errors":          res.Errors,
		"cursor_before":   res.CursorBefore,
		"cursor_after":    res.CursorAfter,
		"skip_reasons":    res.SkipReasons,
		"contract_length": res.ContractLength,
		"caught_up":       res.CaughtUp,
		"until_caught_up": untilCaughtUp,
	}
	if res.LastError != "" {
		out["last_error"] = res.LastError
	}
	writeJSON(w, http.StatusOK, out)
}

// CompatAdminDepositReplay POST 按合约序号重放 skipped → 入账（金额以链上为准）.
func (a *AppService) CompatAdminDepositReplay(w http.ResponseWriter, r *http.Request) {
	indexStr := formOrJSONString(r, "index")
	if indexStr == "" {
		indexStr = r.URL.Query().Get("index")
	}
	if indexStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "index required"})
		return
	}
	index, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil || index < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid index"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	rows, err := getUserInfo(index, index, chainDepositContract)
	if err != nil {
		writeBizError(w, err)
		return
	}
	if len(rows) == 0 || rows[0] == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  biz.ReplayStatusNotFound,
			"index":   index,
			"message": "index not found on chain",
		})
		return
	}
	d := rows[0]
	addr := strings.ToLower(d.Address)
	res, err := a.record.ReplaySkippedChainDeposit(ctx, index, d.Amount, addr)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":               res.Status,
		"index":                res.Index,
		"address":              res.Address,
		"amount":               res.Amount,
		"user_id":              res.UserID,
		"previous_status":      res.PreviousStatus,
		"previous_skip_reason": res.PreviousSkipReason,
		"skip_reason":          res.SkipReason,
	})
}

type chainDepositSyncResult struct {
	Pulled         int
	Credited       int
	Skipped        int
	Errors         int
	CursorBefore   int64
	CursorAfter    int64
	ContractLength int64
	SkipReasons    map[string]int
	LastError      string
	CaughtUp       bool
}

func (a *AppService) syncChainDepositsCatchUp(ctx context.Context) (*chainDepositSyncResult, error) {
	agg := &chainDepositSyncResult{SkipReasons: map[string]int{}}
	globalDeadline := time.Now().Add(chainDepositCatchupWall)
	remaining := chainDepositCatchupMax
	first := true

	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			agg.LastError = err.Error()
			break
		}
		leftWall := time.Until(globalDeadline)
		if leftWall <= 0 {
			break
		}
		roundWall := chainDepositRoundWall
		if leftWall < roundWall {
			roundWall = leftWall
		}
		round, err := a.syncChainDeposits(ctx, chainDepositOpts{
			Chunk:      chainDepositChunk,
			RoundWall:  roundWall,
			MaxIndices: remaining,
		})
		if err != nil {
			return agg, err
		}
		if first {
			agg.CursorBefore = round.CursorBefore
			agg.ContractLength = round.ContractLength
			first = false
		}
		agg.Pulled += round.Pulled
		agg.Credited += round.Credited
		agg.Skipped += round.Skipped
		agg.Errors += round.Errors
		agg.CursorAfter = round.CursorAfter
		agg.ContractLength = round.ContractLength
		if round.LastError != "" {
			agg.LastError = round.LastError
		}
		for k, v := range round.SkipReasons {
			agg.SkipReasons[k] += v
		}
		remaining -= round.Pulled
		agg.CaughtUp = round.CaughtUp
		if round.CaughtUp || round.Pulled == 0 {
			break
		}
	}
	if first {
		// never entered loop — still report cursor
		before, _ := a.record.GetEthUserRecordLast(ctx)
		agg.CursorBefore = before
		agg.CursorAfter = before
		if length, err := getUserLength(chainDepositContract); err == nil {
			agg.ContractLength = length
			agg.CaughtUp = before+1 >= length || length <= 0
		}
	}
	return agg, nil
}

func (a *AppService) syncChainDeposits(ctx context.Context, opts chainDepositOpts) (*chainDepositSyncResult, error) {
	if opts.Chunk <= 0 {
		opts.Chunk = chainDepositChunk
	}
	if opts.RoundWall <= 0 {
		opts.RoundWall = chainDepositRoundWall
	}
	out := &chainDepositSyncResult{SkipReasons: map[string]int{}}
	deadline := time.Now().Add(opts.RoundWall)

	before, err := a.record.GetEthUserRecordLast(ctx)
	if err != nil {
		return nil, err
	}
	out.CursorBefore = before
	out.CursorAfter = before

	userLength, err := getUserLength(chainDepositContract)
	if err != nil {
		return nil, err
	}
	out.ContractLength = userLength
	if userLength <= 0 {
		out.CaughtUp = true
		return out, nil
	}

	start := before + 1
	if start >= userLength {
		out.CaughtUp = true
		return out, nil
	}

	pulledBudget := opts.MaxIndices // 0 = unlimited

	for start < userLength {
		if time.Now().After(deadline) {
			break
		}
		if pulledBudget == 0 && opts.MaxIndices > 0 {
			break
		}
		end := start + int64(opts.Chunk) - 1
		if end >= userLength {
			end = userLength - 1
		}
		// trim batch if MaxIndices limits mid-chunk
		if opts.MaxIndices > 0 && pulledBudget > 0 {
			maxEnd := start + int64(pulledBudget) - 1
			if end > maxEnd {
				end = maxEnd
			}
		}
		batch, err := getUserInfo(start, end, chainDepositContract)
		if err != nil {
			out.LastError = err.Error()
			return out, nil
		}
		if len(batch) == 0 {
			break
		}

		addrs := make([]string, 0, len(batch))
		for _, d := range batch {
			addrs = append(addrs, strings.ToLower(d.Address))
		}
		users, _ := a.record.GetUserByAddress(ctx, addrs...)

		for _, d := range batch {
			if time.Now().After(deadline) {
				out.CaughtUp = out.CursorAfter+1 >= userLength
				return out, nil
			}
			if opts.MaxIndices > 0 && pulledBudget <= 0 {
				out.CaughtUp = out.CursorAfter+1 >= userLength
				return out, nil
			}
			out.Pulled++
			if opts.MaxIndices > 0 {
				pulledBudget--
			}
			addr := strings.ToLower(d.Address)
			u, registered := users[addr]

			var processErr error
			switch {
			case !registered:
				processErr = a.record.ClaimChainDepositSkip(ctx, d.Index, d.Amount, addr, biz.ChainDepositSkipUnreg)
				if processErr == nil {
					out.Skipped++
					out.SkipReasons[biz.ChainDepositSkipUnreg]++
					out.CursorAfter = d.Index
				} else if errors.Is(processErr, biz.ErrChainDepositExists) {
					out.Skipped++
					out.SkipReasons["already"]++
					out.CursorAfter = d.Index
					processErr = nil
				}
			case d.Amount < chainDepositMinAmount:
				processErr = a.record.ClaimChainDepositSkip(ctx, d.Index, d.Amount, addr, biz.ChainDepositSkipBelowMin)
				if processErr == nil {
					out.Skipped++
					out.SkipReasons[biz.ChainDepositSkipBelowMin]++
					out.CursorAfter = d.Index
				} else if errors.Is(processErr, biz.ErrChainDepositExists) {
					out.Skipped++
					out.SkipReasons["already"]++
					out.CursorAfter = d.Index
					processErr = nil
				}
			default:
				processErr = a.record.CreditChainDeposit(ctx, u.ID, d.Index, d.Amount, addr)
				if processErr == nil {
					out.Credited++
					out.CursorAfter = d.Index
				} else if errors.Is(processErr, biz.ErrChainDepositExists) {
					out.Skipped++
					out.SkipReasons["already"]++
					out.CursorAfter = d.Index
					processErr = nil
				}
			}
			if processErr != nil {
				out.Errors++
				out.LastError = fmt.Sprintf("index=%d: %v", d.Index, processErr)
				fmt.Println("chain deposit:", out.LastError)
				if parkErr := a.record.ClaimChainDepositSkip(ctx, d.Index, d.Amount, addr, "error"); parkErr == nil || errors.Is(parkErr, biz.ErrChainDepositExists) {
					out.Skipped++
					out.SkipReasons["error"]++
					out.CursorAfter = d.Index
					processErr = nil
					continue
				}
				return out, nil
			}
		}
		start = end + 1
	}
	out.CaughtUp = out.CursorAfter+1 >= userLength
	return out, nil
}

func getUserLength(address string) (int64, error) {
	var out int64
	err := withBuySomething(address, func(instance *BuySomething) error {
		bals, err := instance.GetUserLength(&bind.CallOpts{})
		if err != nil {
			return err
		}
		out = bals.Int64()
		return nil
	})
	if err != nil {
		return -1, err
	}
	return out, nil
}

func getUserInfo(start int64, end int64, address string) ([]*userDeposit, error) {
	if start > end {
		return nil, nil
	}
	var (
		bals  []common.Address
		bals2 []*big.Int
		bals3 []*big.Int
	)
	err := withBuySomething(address, func(instance *BuySomething) error {
		opts := &bind.CallOpts{}
		startBI := new(big.Int).SetInt64(start)
		endBI := new(big.Int).SetInt64(end)
		var err error
		bals, err = instance.GetUsersByIndex(opts, startBI, endBI)
		if err != nil {
			return err
		}
		bals2, err = instance.GetUsersAmountByIndex(opts, startBI, endBI)
		if err != nil {
			return err
		}
		bals3, err = instance.GetIdsByIndex(opts, startBI, endBI)
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(bals) != len(bals2) || len(bals) != len(bals3) {
		return nil, fmt.Errorf("chain deposit batch length mismatch users=%d amounts=%d ids=%d", len(bals), len(bals2), len(bals3))
	}

	users := make([]*userDeposit, 0, len(bals))
	for k, v := range bals {
		users = append(users, &userDeposit{
			Index:   start + int64(k),
			Address: v.String(),
			Amount:  amountAsUSDT(bals2[k]),
			Id:      bals3[k].Int64(),
		})
	}
	return users, nil
}

// withBuySomething dials BSC RPCs in order and runs fn against BuySomething.
func withBuySomething(contract string, fn func(*BuySomething) error) error {
	var lastErr error
	for _, url1 := range bscRPCURLs() {
		client, err := ethclient.Dial(url1)
		if err != nil {
			lastErr = err
			continue
		}
		instance, err := NewBuySomething(common.HexToAddress(contract), client)
		if err != nil {
			lastErr = err
			client.Close()
			continue
		}
		err = fn(instance)
		client.Close()
		if err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all bsc rpc failed")
	}
	return lastErr
}

func amountAsUSDT(v *big.Int) int64 {
	if v == nil {
		return 0
	}
	wei := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	if v.Cmp(wei) >= 0 {
		return new(big.Int).Div(v, wei).Int64()
	}
	return v.Int64()
}

func bscRPCURLs() []string {
	return []string{
		"https://bsc-dataseed4.binance.org/",
		"https://binance.llamarpc.com/",
		"https://bscrpc.com/",
		"https://bsc-pokt.nodies.app/",
		"https://bsc-dataseed.binance.org/",
	}
}
