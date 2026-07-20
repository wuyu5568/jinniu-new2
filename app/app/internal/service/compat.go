package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jinniu/app/app/app/internal/biz"
	"github.com/jinniu/app/app/app/internal/pkg/middleware/auth"
	"github.com/shopspring/decimal"
)

const defaultPageSize = 10

var jinniuMenuPaths = []string{
	"/home", "/ordersList", "/recharge", "/member",
	"/subscription", "/withdraw", "/config", "/lookChildren",
}

func parsePage(r *http.Request) int {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		return 1
	}
	return page
}

func decStr(d decimal.Decimal) string {
	return d.StringFixed(2)
}

// decStrFloor4 formats with 4 decimal places by truncating (no round).
func decStrFloor4(d decimal.Decimal) string {
	return d.Truncate(4).StringFixed(4)
}

func zeroStr() string { return "0.00" }

func compatFail(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "fail"})
}

func compatFailMsg(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "fail", "message": msg})
}

// --- User compat (taurus /api/app_server) ---

func (s *AppService) CompatEthAuthorize(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Address   string `json:"address"`
		Code      string `json:"code"`
		Sign      string `json:"sign"`
		Signature string `json:"signature"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		compatFail(w)
		return
	}
	sign := body.Sign
	if sign == "" {
		sign = body.Signature
	}
	res, err := s.users.EthAuthorize(r.Context(), body.Address, sign, body.Code)
	if err != nil {
		switch {
		case errors.Is(err, biz.ErrUserDisabled):
			writeJSON(w, http.StatusOK, map[string]string{"status": "用户已锁定"})
		case errors.Is(err, biz.ErrInviteRequired):
			writeJSON(w, http.StatusOK, map[string]string{"status": "请输入推荐码"})
		case errors.Is(err, biz.ErrInviteInvalid):
			writeJSON(w, http.StatusOK, map[string]string{"status": "无效的推荐码"})
		default:
			compatFail(w)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "token": res.Token})
}

// CompatSubscribeTiers returns preset subscribe amounts (public, no auth).
func (s *AppService) CompatSubscribeTiers(w http.ResponseWriter, _ *http.Request) {
	params := biz.GetActiveParams()
	tiers, err := biz.ParseSubscribeTiers(params.SubscribeTiers)
	if err != nil {
		writeBizError(w, err)
		return
	}
	amounts := make([]string, 0, len(tiers))
	for _, t := range tiers {
		amounts = append(amounts, t.String())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tiers":                amounts,
		"min_subscribe_amount": params.MinSubscribeAmount,
	})
}

func (s *AppService) CompatUserInfo(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	user, err := s.users.GetProfile(r.Context(), uid)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.buildTaurusUserInfo(r.Context(), user))
}

func (s *AppService) buildTaurusUserInfo(ctx context.Context, user *biz.User) map[string]any {
	params := biz.GetActiveParams()
	withdrawRate := params.ExtractFeeRate
	withdrawMin := "0"

	inviteAddr := ""
	if user.InviterID != nil {
		if inv, err := s.record.GetUserByID(ctx, *user.InviterID); err == nil && inv != nil {
			inviteAddr = inv.Address
		}
	}

	activeSum, _ := s.record.SumActiveAmount(ctx, user.ID)
	directs, _ := s.record.CountEffectiveDirectReferrals(ctx, user.ID)

	remaining := decimal.Zero
	activeLocCount := 0
	locationList := make([]map[string]any, 0)
	if locs, err := s.record.ListLocations(ctx, user.ID, biz.OrderStatusActive); err == nil {
		activeLocCount = len(locs)
		for _, loc := range locs {
			locationList = append(locationList, compatLocationItem(loc))
			left := loc.ExitTarget.Sub(loc.Accumulated)
			if left.IsPositive() {
				remaining = remaining.Add(left)
			}
		}
	}

	teamTotal, maxLeg, smallArea, _ := s.record.AreaVolumes(ctx, user.ID)
	areaMin := smallArea
	if areaMin.IsZero() {
		areaMin = user.CommunityVolume
	}

	from := time.Now().AddDate(-10, 0, 0)
	to := time.Now().AddDate(0, 0, 1)
	ledgers, _ := s.record.ListLedger(ctx, user.ID, from, to)
	var staticSum, genSum, communitySum, peerSum decimal.Decimal
	for _, e := range ledgers {
		switch e.EntryType {
		case biz.LedgerStatic:
			staticSum = staticSum.Add(e.Amount)
		case biz.LedgerGeneration:
			genSum = genSum.Add(e.Amount)
		case biz.LedgerCommunityBase:
			communitySum = communitySum.Add(e.Amount)
		case biz.LedgerPeer:
			peerSum = peerSum.Add(e.Amount)
		}
	}
	allReward := staticSum.Add(genSum).Add(communitySum).Add(peerSum)

	base := map[string]any{
		"status":            "ok",
		"address":           user.Address,
		"level":             strconv.Itoa(int(user.CommunityLevel)),
		"usdt":              decStr(user.AccountBalance),
		"raw":               decStr(user.AccountBalance),
		"amountGet":         decStrFloor4(user.WithdrawableBalance),
		"amountUsdt":        decStr(user.AccountBalance),
		"inviteUserAddress": inviteAddr,
		"withdrawRate":      withdrawRate,
		"withdrawMin":       withdrawMin,
		"withdrawRateTwo":   withdrawRate,
		"withdrawMinTwo":    withdrawMin,
		"locationNum":       strconv.Itoa(activeLocCount),
		"LocationList":      locationList,
		"total":             decStr(teamTotal),
		"max":               decStr(maxLeg),
		"min":               decStr(areaMin),
		"buy":               decStr(activeSum),
		"amountGetSub":      decStr(remaining),
		"outNum":            "0",
		"location":          decStr(staticSum),
		"recommend":         decStr(genSum),
		"recommendTwo":      zeroStr(),
		"team":              decStr(communitySum),
		"teamTwo":           decStr(peerSum),
		"all":               decStr(allReward),
		"recommendNum":      strconv.Itoa(directs),
		"notice":            "",
		"goods":             []any{},
		"one":               zeroStr(),
		"two":               zeroStr(),
		"three":             zeroStr(),
		"four":              zeroStr(),
		"five":              zeroStr(),
		"six":               zeroStr(),
		"seven":             zeroStr(),
	}
	return base
}

func (s *AppService) CompatBuy(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	var body struct {
		Amount json.Number `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		compatFail(w)
		return
	}
	amount, err := decimal.NewFromString(body.Amount.String())
	if err != nil {
		compatFail(w)
		return
	}
	loc, err := s.record.CreateLocation(r.Context(), uid, amount)
	if err != nil {
		compatFail(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"id":     loc.ID,
		"amount": loc.Amount.String(),
	})
}

func (s *AppService) CompatOrderList(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	page := parsePage(r)
	locs, err := s.record.ListLocations(r.Context(), uid, "")
	if err != nil {
		writeBizError(w, err)
		return
	}
	list := make([]map[string]any, 0, len(locs))
	for _, l := range locs {
		list = append(list, compatLocationItem(l))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(list),
		"list":  paginateSlice(list, page, defaultPageSize),
	})
}

func (s *AppService) CompatLocationList(w http.ResponseWriter, r *http.Request) {
	s.CompatOrderList(w, r)
}

func compatLocationItem(l *biz.Location) map[string]any {
	status := "1"
	if l.Status == biz.OrderStatusExited {
		status = "2"
	}
	return map[string]any{
		"id":           l.ID,
		"amount":       l.Amount.String(),
		"status":       status,
		"accumulated":  l.Accumulated.String(),
		"exit_target":  l.ExitTarget.String(),
		"rate_percent": l.RatePercent.String(),
		"createdAt":    l.CreatedAt.Format("2006-01-02 15:04:05"),
		"created_at":   l.CreatedAt.Format(time.RFC3339),
	}
}

func (s *AppService) CompatWithdraw(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	var body map[string]any
	if err := dec.Decode(&body); err != nil {
		compatFailMsg(w, "参数错误")
		return
	}
	amount, err := compatParseDecimal(body["amount"])
	if err != nil {
		compatFailMsg(w, "金额无效")
		return
	}
	wd, err := s.record.CreateWithdraw(r.Context(), uid, amount, nil)
	if err != nil {
		compatFailMsg(w, compatWithdrawMessage(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": wd.ID})
}

// compatParseDecimal accepts JSON number, string, or null-ish amount from legacy frontends.
func compatParseDecimal(v any) (decimal.Decimal, error) {
	if v == nil {
		return decimal.Zero, errors.New("empty amount")
	}
	switch t := v.(type) {
	case json.Number:
		s := strings.TrimSpace(t.String())
		if s == "" {
			return decimal.Zero, errors.New("empty amount")
		}
		return decimal.NewFromString(s)
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return decimal.Zero, errors.New("empty amount")
		}
		return decimal.NewFromString(s)
	case float64:
		return decimal.NewFromFloat(t), nil
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		if s == "" || s == "<nil>" {
			return decimal.Zero, errors.New("empty amount")
		}
		return decimal.NewFromString(s)
	}
}

func (s *AppService) CompatWithdrawCancel(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	var body struct {
		ID json.Number `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	id, _ := body.ID.Int64()
	if id == 0 {
		id = int64(formOrJSONUint64(r, "id"))
	}
	if id == 0 {
		compatFailMsg(w, "参数错误")
		return
	}
	if _, err := s.record.CancelWithdraw(r.Context(), uid, uint64(id)); err != nil {
		msg := "取消失败"
		if errors.Is(err, biz.ErrWithdrawCancelDisabled) {
			msg = "提现确认后不可取消"
		} else if errors.Is(err, biz.ErrWithdrawConflict) {
			msg = "仅待审订单可取消"
		} else if errors.Is(err, biz.ErrWithdrawNotFound) {
			msg = "提现单不存在"
		}
		compatFailMsg(w, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func compatWithdrawMessage(err error) string {
	switch {
	case errors.Is(err, biz.ErrInsufficientBalance):
		return "可提余额不足"
	case errors.Is(err, biz.ErrInvalidAmount):
		return "金额无效"
	case errors.Is(err, biz.ErrPayoutDisabled), errors.Is(err, biz.ErrPayoutNotConfigured), errors.Is(err, biz.ErrPayoutMaxRequired):
		return "提现暂不可用"
	case errors.Is(err, biz.ErrPayoutAboveMax):
		return "超过单笔打款上限"
	default:
		return "提现失败"
	}
}

func (s *AppService) CompatWithdrawList(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	page := parsePage(r)
	items, err := s.record.ListWithdraws(r.Context(), uid, "")
	if err != nil {
		writeBizError(w, err)
		return
	}
	list := make([]map[string]any, 0, len(items))
	for _, w := range items {
		list = append(list, map[string]any{
			"id":        w.ID,
			"amount":    w.Amount.String(),
			"feeAmount": w.FeeAmount.String(),
			"relAmount": w.CreditedAmount.String(),
			"status":    mapWithdrawStatus(w.Status),
			"tx_hash":   w.TxHash,
			"createdAt": w.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(list),
		"list":  paginateSlice(list, page, defaultPageSize),
	})
}

func (s *AppService) CompatRecommendList(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Query().Get("address")
	refs, err := s.record.ListDirectReferrals(r.Context(), addr)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"recommends": []any{}})
		return
	}
	out := make([]map[string]any, 0, len(refs))
	for _, u := range refs {
		amount, _ := s.record.SumActiveAmount(r.Context(), u.ID)
		activated, _ := s.record.UserHasLocation(r.Context(), u.ID)
		effective, _ := s.record.CountEffectiveDirectReferrals(r.Context(), u.ID)
		registered, _ := s.record.CountDirectReferrals(r.Context(), u.ID)
		out = append(out, map[string]any{
			"address":     u.Address,
			"amount":      decStr(amount),
			"countLow":    effective,
			"activated":   activated,
			"hasChildren": registered > 0,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"recommends": out})
}

func (s *AppService) CompatRewardList(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	page := parsePage(r)
	reqType := r.URL.Query().Get("reqType")
	from, to := ledgerRange(r)
	// Wider default window for asset page history.
	if r.URL.Query().Get("from") == "" {
		from = time.Now().AddDate(-1, 0, 0)
		to = time.Now().AddDate(0, 0, 1)
	}
	items, err := s.record.ListLedger(r.Context(), uid, from, to)
	if err != nil {
		writeBizError(w, err)
		return
	}
	want := reqTypeToEntryType(reqType)
	list := make([]map[string]any, 0, len(items))
	for _, e := range items {
		if want != "" && e.EntryType != want {
			continue
		}
		// Skip non-earnings types unless explicitly requested.
		if want == "" && e.EntryType != biz.LedgerStatic && e.EntryType != biz.LedgerGeneration &&
			e.EntryType != biz.LedgerCommunityBase && e.EntryType != biz.LedgerPeer {
			continue
		}
		addr, num := parseGenerationRemark(e.Remark)
		list = append(list, map[string]any{
			"id":        e.ID,
			"amount":    e.Amount.String(),
			"amountTwo": e.Amount.String(),
			"reward":    e.Amount.String(),
			"name":      ledgerNameCompat(e.EntryType),
			"address":   addr,
			"num":       num,
			"reason":    ledgerReasonCompat(e.EntryType),
			"createdAt": e.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(list),
		"list":  paginateSlice(list, page, defaultPageSize),
	})
}

// parseGenerationRemark extracts source address and depth from
// "generation from=<addr> depth=<n>" remarks; otherwise returns empty strings.
func parseGenerationRemark(remark string) (address, depth string) {
	if remark == "" {
		return "", ""
	}
	parts := strings.Fields(remark)
	for _, p := range parts {
		if strings.HasPrefix(p, "from=") {
			address = strings.TrimPrefix(p, "from=")
		}
		if strings.HasPrefix(p, "depth=") {
			depth = strings.TrimPrefix(p, "depth=")
		}
	}
	return address, depth
}

// parseLedgerKV reads key=value tokens from a ledger remark.
func parseLedgerKV(remark, key string) string {
	if remark == "" || key == "" {
		return ""
	}
	prefix := key + "="
	for _, p := range strings.Fields(remark) {
		if strings.HasPrefix(p, prefix) {
			return strings.TrimPrefix(p, prefix)
		}
	}
	return ""
}

func formatStaticRate(amount, buyAmount decimal.Decimal) string {
	if !buyAmount.IsPositive() || !amount.IsPositive() {
		return ""
	}
	return amount.Div(buyAmount).Mul(decimal.NewFromInt(100)).Round(4).String()
}

func formatGapRate(amount, sourceStatic decimal.Decimal, gapFromRemark string) string {
	if gapFromRemark != "" {
		if g, err := decimal.NewFromString(gapFromRemark); err == nil && g.IsPositive() {
			return formatRateFraction(g)
		}
	}
	if sourceStatic.IsPositive() && amount.IsPositive() {
		return formatRateFraction(amount.Div(sourceStatic))
	}
	return ""
}

func formatRateFraction(frac decimal.Decimal) string {
	if !frac.IsPositive() {
		return ""
	}
	return frac.Mul(decimal.NewFromInt(100)).Round(2).StringFixed(2) + "%"
}

func buildCalcDetail(sourceStatic, applyRate, amount string) string {
	if sourceStatic == "" || applyRate == "" || amount == "" {
		return ""
	}
	return sourceStatic + " × " + applyRate + " = " + amount
}

func formatVipLabel(vipTok string) string {
	if vipTok == "" {
		return ""
	}
	return "V" + vipTok
}

func reqTypeToEntryType(reqType string) string {
	switch reqType {
	case "2":
		return biz.LedgerStatic
	case "3":
		return biz.LedgerGeneration
	case "5":
		return biz.LedgerCommunityBase
	case "6":
		return biz.LedgerPeer
	default:
		return ""
	}
}

func (s *AppService) CompatDepositList(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	page := parsePage(r)
	from := time.Now().AddDate(-1, 0, 0)
	to := time.Now().AddDate(0, 0, 1)
	items, err := s.record.ListLedger(r.Context(), uid, from, to)
	if err != nil {
		writeBizError(w, err)
		return
	}
	list := make([]map[string]any, 0)
	for _, e := range items {
		if e.EntryType != biz.LedgerDeposit {
			continue
		}
		list = append(list, map[string]any{
			"amount":    e.Amount.String(),
			"createdAt": e.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(list),
		"list":  paginateSlice(list, page, defaultPageSize),
	})
}

// --- Admin compat (dapp-admin /api/admin_jinniu) ---

func (s *AppService) CompatAdminLogin(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	username := r.FormValue("username")
	if username == "" {
		username = r.FormValue("account")
	}
	if username == "" {
		username = r.FormValue("email")
	}
	password := r.FormValue("password")
	if username == "" {
		var body struct {
			Username string `json:"username"`
			Account  string `json:"account"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		username, password = body.Username, body.Password
		if username == "" {
			username = body.Account
		}
		if username == "" {
			username = body.Email
		}
	}
	token, err := s.users.AdminLogin(r.Context(), username, password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "invalid credentials"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *AppService) CompatMyAuthList(w http.ResponseWriter, _ *http.Request) {
	authItems := make([]map[string]string, len(jinniuMenuPaths))
	for i, p := range jinniuMenuPaths {
		authItems[i] = map[string]string{"path": p}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"super": "1",
		"auth":  authItems,
	})
}

func (s *AppService) CompatAdminAll(w http.ResponseWriter, r *http.Request) {
	stats, err := s.record.DashboardStats(r.Context(), time.Now())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	out := map[string]any{
		"totalUserR":    stats.TotalUserR,
		"totalUser":     stats.TotalUser,
		"todayUserR":    stats.TodayUserR,
		"todayUser":     stats.TodayUser,
		"buyTotal":      decStr(stats.BuyTotal),
		"todayBuy":      decStr(stats.TodayBuy),
		"balanceUsdt":   decStr(stats.BalanceUsdt),
		"todayOne":      decStr(stats.TodayOne),
		"todayTwo":      decStr(stats.TodayTwo),
		"todayThree":    decStr(stats.TodayThree),
		"totalReward":   decStr(stats.TotalReward),
		"todayWithdraw": decStr(stats.TodayWithdraw),
		"totalWithdraw": decStr(stats.TotalWithdraw),
		"totalIspay":    decStr(stats.TotalIspay),
	}
	if st, err := s.record.SettleStatus(r.Context()); err == nil && st != nil {
		out["settle_today"] = st.TodayDate
		out["settle_today_done"] = st.TodaySettled
		out["allow_force_settle"] = st.AllowForce
		if st.Today != nil {
			out["settle_today_static"] = st.Today.SettledCount
			out["settle_today_gen"] = st.Today.GenerationCount
			out["settle_today_community"] = st.Today.CommunityCount
			out["settle_today_peer"] = st.Today.PeerCount
			out["settle_today_forced"] = st.Today.Forced
			out["settle_today_updated_at"] = st.Today.UpdatedAt.Format("2006-01-02 15:04:05")
		}
		if st.Latest != nil {
			out["settle_latest_date"] = st.Latest.SettleDate.Format("2006-01-02")
			out["settle_latest_static"] = st.Latest.SettledCount
			out["settle_latest_updated_at"] = st.Latest.UpdatedAt.Format("2006-01-02 15:04:05")
		}
	}
	if ps, err := s.record.PayoutStatus(r.Context()); err == nil && ps != nil {
		for k, v := range payoutStatusMap(ps) {
			out[k] = v
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func payoutStatusMap(ps *biz.PayoutStatusView) map[string]any {
	return map[string]any{
		"payout_enabled":          ps.Enabled,
		"payout_key_configured":   ps.KeyConfigured,
		"payout_hot_address":      ps.HotAddress,
		"payout_max_usdt":         ps.MaxUSDT,
		"payout_cron":             ps.PayoutCron,
		"payout_queue_rewarded":   ps.QueueRewarded,
		"payout_queue_doing":      ps.QueueDoing,
		"payout_max_required_ok":  ps.MaxRequiredOK,
		"payout_allow_force_settle": ps.AllowForceSettle,
	}
}

func (s *AppService) CompatAdminPayoutStatus(w http.ResponseWriter, r *http.Request) {
	ps, err := s.record.PayoutStatus(r.Context())
	if err != nil {
		writeBizError(w, err)
		return
	}
	out := payoutStatusMap(ps)
	out["enabled"] = ps.Enabled
	out["key_configured"] = ps.KeyConfigured
	out["hot_address"] = ps.HotAddress
	out["max_usdt"] = ps.MaxUSDT
	out["queue_rewarded"] = ps.QueueRewarded
	out["queue_doing"] = ps.QueueDoing
	out["max_required_ok"] = ps.MaxRequiredOK
	out["allow_force_settle"] = ps.AllowForceSettle
	writeJSON(w, http.StatusOK, out)
}

func (s *AppService) CompatAdminSettleStatus(w http.ResponseWriter, r *http.Request) {
	st, err := s.record.SettleStatus(r.Context())
	if err != nil {
		writeBizError(w, err)
		return
	}
	out := map[string]any{
		"settle_today":       st.TodayDate,
		"settle_today_done":  st.TodaySettled,
		"allow_force_settle": st.AllowForce,
	}
	if st.Today != nil {
		out["today"] = map[string]any{
			"settle_date":      st.Today.SettleDate.Format("2006-01-02"),
			"forced":           st.Today.Forced,
			"settled_count":    st.Today.SettledCount,
			"exited_count":     st.Today.ExitedCount,
			"generation_count": st.Today.GenerationCount,
			"community_count":  st.Today.CommunityCount,
			"peer_count":       st.Today.PeerCount,
			"remark":           st.Today.Remark,
			"updated_at":       st.Today.UpdatedAt.Format(time.RFC3339),
		}
	}
	if st.Latest != nil {
		out["latest"] = map[string]any{
			"settle_date":      st.Latest.SettleDate.Format("2006-01-02"),
			"forced":           st.Latest.Forced,
			"settled_count":    st.Latest.SettledCount,
			"exited_count":     st.Latest.ExitedCount,
			"generation_count": st.Latest.GenerationCount,
			"community_count":  st.Latest.CommunityCount,
			"peer_count":       st.Latest.PeerCount,
			"remark":           st.Latest.Remark,
			"updated_at":       st.Latest.UpdatedAt.Format(time.RFC3339),
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *AppService) CompatAdminUserList(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r)
	address := r.URL.Query().Get("address")
	users, total, err := s.record.ListUsersAdmin(r.Context(), address, page, defaultPageSize)
	if err != nil {
		writeBizError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		activeSum, _ := s.record.SumActiveAmount(r.Context(), u.ID)
		exitRemain, _ := s.record.SumActiveExitRemaining(r.Context(), u.ID)
		teamTotal, maxLeg, smallArea, _ := s.record.AreaVolumes(r.Context(), u.ID)
		inviteAddr := ""
		if u.InviterID != nil {
			if inv, err := s.record.GetUserByID(r.Context(), *u.InviterID); err == nil {
				inviteAddr = inv.Address
			}
		}
		lock := "0"
		if u.IsDisabled() {
			lock = "1"
		}
		lockReward := "0"
		if u.RewardLocked {
			lockReward = "1"
		}
		vipLocked := "0"
		if u.CommunityLevelLocked {
			vipLocked = "1"
		}
		directs, _ := s.record.CountEffectiveDirectReferrals(r.Context(), u.ID)
		out = append(out, map[string]any{
			"userId":              strconv.FormatUint(u.ID, 10),
			"createdAt":           u.CreatedAt.Format("2006-01-02 15:04:05"),
			"address":             u.Address,
			"amountUsdtCurrent":   decStr(activeSum),
			"exitRemain":          decStr(exitRemain),
			"bAmount":             zeroStr(),
			"amountUsdtGet":       decStrFloor4(u.WithdrawableBalance),
			"amountUsdtTwo":       zeroStr(),
			"balanceUsdt":         decStr(u.AccountBalance),
			"balanceDhb":          zeroStr(),
			"bAmountTwo":          zeroStr(),
			"perDayAmount":        zeroStr(),
			"out":                 "0",
			"areaTeam":            decStr(teamTotal),
			"areaMax":             decStr(maxLeg),
			"areaTotal":           decStr(smallArea),
			"areaMin":             zeroStr(),
			"vip":                 strconv.Itoa(int(u.CommunityLevel)),
			"vipLocked":           vipLocked,
			"historyRecommend":    strconv.Itoa(directs),
			"lock":                lock,
			"lockReward":          lockReward,
			"myRecommendAddress":  inviteAddr,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out, "count": strconv.Itoa(total)})
}

func (s *AppService) CompatAdminLocationList(w http.ResponseWriter, r *http.Request) {
	s.compatAdminBuyList(w, r)
}

func (s *AppService) CompatAdminBuyList(w http.ResponseWriter, r *http.Request) {
	s.compatAdminBuyList(w, r)
}

func (s *AppService) compatAdminBuyList(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r)
	address := r.URL.Query().Get("address")
	locs, total, err := s.record.ListLocationsAdmin(r.Context(), address, page, defaultPageSize)
	if err != nil {
		writeBizError(w, err)
		return
	}
	rewards := make([]map[string]any, 0, len(locs))
	for _, l := range locs {
		addr := ""
		if u, err := s.record.GetUserByID(r.Context(), l.UserID); err == nil {
			addr = u.Address
		}
		rewards = append(rewards, map[string]any{
			"id":            l.ID,
			"amount":        l.Amount.String(),
			"address":       addr,
			"createdAt":     l.CreatedAt.Format("2006-01-02 15:04:05"),
			"status":        l.Status,
			"multiplier":    l.Multiplier.String(),
			"exitTarget":    l.ExitTarget.String(),
			"accumulated":   l.Accumulated.String(),
			"ratePercent":   l.RatePercent.String(),
			"rateDirection": l.RateDirection,
			"one":           l.Status, // legacy column reuse avoided by FE rewrite
			"two":           "",
			"three":         "",
			"four":          "",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"rewards": rewards, "count": strconv.Itoa(total)})
}

func (s *AppService) CompatAdminWithdrawList(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r)
	address := r.URL.Query().Get("address")
	items, err := s.record.ListWithdrawsAdmin(r.Context(), "", 0)
	if err != nil {
		writeBizError(w, err)
		return
	}
	filtered := make([]*biz.Withdraw, 0)
	for _, w := range items {
		if address != "" {
			u, err := s.record.GetUserByID(r.Context(), w.UserID)
			if err != nil || u == nil || !containsFold(u.Address, address) {
				continue
			}
		}
		filtered = append(filtered, w)
	}
	out := make([]map[string]any, 0)
	for _, w := range filtered {
		addr := ""
		if u, err := s.record.GetUserByID(r.Context(), w.UserID); err == nil {
			addr = u.Address
		}
		out = append(out, map[string]any{
			"id":           w.ID,
			"address":      addr,
			"relAmount":    w.CreditedAmount.String(),
			"amount":       w.Amount.String(),
			"feeAmount":    w.FeeAmount.String(),
			"status":       mapWithdrawStatus(w.Status),
			"tx_hash":      w.TxHash,
			"payout_error": w.PayoutError,
			"createdAt":    w.CreatedAt.Format("2006-01-02 15:04:05"),
			"remark":       w.Remark,
		})
	}
	paged := paginateSlice(out, page, defaultPageSize)
	writeJSON(w, http.StatusOK, map[string]any{
		"withdraw": paged,
		"count":    strconv.Itoa(len(filtered)),
	})
}

func (s *AppService) CompatAdminWithdrawPass(w http.ResponseWriter, r *http.Request) {
	compatFailMsg(w, "提现已取消审核，确认即打款")
}

func (s *AppService) CompatAdminWithdrawReject(w http.ResponseWriter, r *http.Request) {
	compatFailMsg(w, "提现已取消审核，确认即打款")
}

// CompatAdminWithdrawCancelPending refunds legacy pending withdraws (ADR 0012 one-shot).
func (s *AppService) CompatAdminWithdrawCancelPending(w http.ResponseWriter, r *http.Request) {
	n, err := s.record.CancelAllPendingWithdraws(r.Context())
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "cancelled": n})
}

func (s *AppService) CompatAdminWithdrawPayout(w http.ResponseWriter, r *http.Request) {
	id := formOrJSONUint64(r, "id")
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid id"})
		return
	}
	wd, err := s.record.ProcessWithdrawPayout(r.Context(), id)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"id":           wd.ID,
		"withdraw_status": wd.Status,
		"tx_hash":      wd.TxHash,
		"payout_error": wd.PayoutError,
	})
}

func (s *AppService) CompatAdminWithdrawPayoutConfirm(w http.ResponseWriter, r *http.Request) {
	id := formOrJSONUint64(r, "id")
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid id"})
		return
	}
	wd, err := s.record.ConfirmWithdrawPayout(r.Context(), id)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"id":              wd.ID,
		"withdraw_status": wd.Status,
		"tx_hash":         wd.TxHash,
		"payout_error":    wd.PayoutError,
	})
}

func (s *AppService) CompatAdminWithdrawPayoutRun(w http.ResponseWriter, r *http.Request) {
	n, err := s.record.RunPayoutQueue(r.Context(), 20)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "processed": n})
}

func (s *AppService) CompatAdminSettle(w http.ResponseWriter, r *http.Request) {
	forceStr := formOrJSONString(r, "force")
	if forceStr == "" {
		forceStr = r.URL.Query().Get("force")
	}
	force := forceStr == "1" || forceStr == "true"
	res, err := s.record.SettleStatic(r.Context(), nil, force)
	if err != nil {
		writeBizError(w, err)
		return
	}
	status := "ok"
	if res.Skipped {
		status = "already_settled"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           status,
		"skipped":          res.Skipped,
		"forced":           res.Forced,
		"settle_date":      res.SettleDate,
		"settled_count":    res.SettledCount,
		"exited_count":     res.ExitedCount,
		"generation_count": res.GenerationCount,
		"community_count":  res.CommunityCount,
		"peer_count":       res.PeerCount,
		"warning":          settleForceWarning(res),
	})
}

func settleForceWarning(res *biz.SettleResult) string {
	if res != nil && res.Forced && !res.Skipped {
		return "force=1 may double-credit; test only"
	}
	return ""
}

func (s *AppService) CompatAdminConfig(w http.ResponseWriter, r *http.Request) {
	items, err := s.record.ListConfigs(r.Context())
	if err != nil {
		writeBizError(w, err)
		return
	}
	config := make([]map[string]any, len(items))
	for i, c := range items {
		config[i] = map[string]any{
			"id":    c.ID,
			"key":   c.Key,
			"name":  c.Name,
			"value": c.Value,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": config})
}

func (s *AppService) CompatAdminConfigUpdate(w http.ResponseWriter, r *http.Request) {
	id := formOrJSONUint64(r, "id")
	value := formOrJSONString(r, "value")
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid id"})
		return
	}
	if _, err := s.record.UpdateConfig(r.Context(), id, value); err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *AppService) CompatAdminRewardList(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r)
	address := r.URL.Query().Get("address")
	reason := r.URL.Query().Get("reason")
	entryType := ledgerEntryFromReason(reason)
	items, total, err := s.record.ListLedgerAdmin(r.Context(), address, entryType, page, defaultPageSize)
	if err != nil {
		writeBizError(w, err)
		return
	}
	rewards := make([]map[string]any, 0, len(items))
	for _, e := range items {
		addr := ""
		if u, err := s.record.GetUserByID(r.Context(), e.UserID); err == nil {
			addr = u.Address
		}
		srcAddr, depth := parseGenerationRemark(e.Remark)
		if srcAddr == "" {
			srcAddr = parseLedgerKV(e.Remark, "from")
		}
		buyAmount := ""
		staticRate := ""
		gapRate := ""
		sourceStatic := ""
		applyRate := ""
		calcDetail := ""
		amtStr := e.Amount.String()
		switch e.EntryType {
		case biz.LedgerStatic:
			if e.OrderID != nil {
				if loc, err := s.record.GetLocationByID(r.Context(), *e.OrderID); err == nil && loc != nil {
					buyAmount = loc.Amount.String()
				}
			}
			if buyAmount != "" {
				if buy, err := decimal.NewFromString(buyAmount); err == nil {
					staticRate = formatStaticRate(e.Amount, buy)
				}
			}
			if staticRate != "" && buyAmount != "" {
				calcDetail = buyAmount + " × " + staticRate + "% = " + amtStr
			}
		case biz.LedgerCommunityBase, biz.LedgerGeneration:
			// 认购金额 / 静态利率 = 来源那笔静态对应的认购与利率（非领取人填单）
			sourceStatic = parseLedgerKV(e.Remark, "static")
			buyAmount = parseLedgerKV(e.Remark, "buy")
			ratePctTok := parseLedgerKV(e.Remark, "ratePct")
			if ratePctTok != "" {
				staticRate = ratePctTok
			} else if sourceStatic != "" && buyAmount != "" {
				if ss, err1 := decimal.NewFromString(sourceStatic); err1 == nil {
					if buy, err2 := decimal.NewFromString(buyAmount); err2 == nil {
						staticRate = formatStaticRate(ss, buy)
					}
				}
			}
			switch e.EntryType {
			case biz.LedgerCommunityBase:
				gapTok := parseLedgerKV(e.Remark, "gap")
				srcStaticDec := decimal.Zero
				if sourceStatic != "" {
					srcStaticDec, _ = decimal.NewFromString(sourceStatic)
				}
				gapRate = formatGapRate(e.Amount, srcStaticDec, gapTok)
				applyRate = gapRate
			case biz.LedgerGeneration:
				rateTok := parseLedgerKV(e.Remark, "rate")
				if rateTok == "" {
					rateTok = biz.GetActiveParams().GenerationRate
				}
				if rdec, err := decimal.NewFromString(rateTok); err == nil {
					applyRate = formatRateFraction(rdec)
				}
			}
			if sourceStatic != "" && applyRate != "" {
				calcDetail = buildCalcDetail(sourceStatic, applyRate, amtStr)
			}
		case biz.LedgerPeer:
			// 平级单笔：基数为来源用户当日社区基础奖合计；新 remark 用 community=，旧数据仍可能是 static=
			sourceStatic = parseLedgerKV(e.Remark, "community")
			if sourceStatic == "" {
				sourceStatic = parseLedgerKV(e.Remark, "static")
			}
			rateTok := parseLedgerKV(e.Remark, "rate")
			if rdec, err := decimal.NewFromString(rateTok); err == nil {
				applyRate = formatRateFraction(rdec)
			}
			if sourceStatic != "" && applyRate != "" {
				calcDetail = buildCalcDetail(sourceStatic, applyRate, amtStr)
			}
		}
		vip := formatVipLabel(parseLedgerKV(e.Remark, "vip"))
		fromVip := formatVipLabel(parseLedgerKV(e.Remark, "fromVip"))
		rewards = append(rewards, map[string]any{
			"createdAt":     e.CreatedAt.Format("2006-01-02 15:04:05"),
			"amount":        amtStr,
			"buyAmount":     buyAmount,
			"staticRate":    staticRate,
			"gapRate":       gapRate,
			"sourceStatic":  sourceStatic,
			"applyRate":     applyRate,
			"calcDetail":    calcDetail,
			"address":       addr,
			"vip":           vip,
			"reason":        ledgerReasonCompat(e.EntryType),
			"name":          ledgerNameCompat(e.EntryType),
			"addressTwo":    srcAddr,
			"fromVip":       fromVip,
			"num":           depth,
			"remark":        e.Remark,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"rewards": rewards, "count": strconv.Itoa(total)})
}

func (s *AppService) CompatAdminRecordList(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r)
	address := r.URL.Query().Get("address")
	items, total, err := s.record.ListLedgerAdmin(r.Context(), address, biz.LedgerDeposit, page, defaultPageSize)
	if err != nil {
		writeBizError(w, err)
		return
	}
	locations := make([]map[string]any, 0, len(items))
	for _, e := range items {
		addr := ""
		if u, err := s.record.GetUserByID(r.Context(), e.UserID); err == nil {
			addr = u.Address
		}
		locations = append(locations, map[string]any{
			"address":   addr,
			"amount":    e.Amount.String(),
			"createdAt": e.CreatedAt.Format("2006-01-02 15:04:05"),
			"remark":    e.Remark,
			"type":      "account",
			"typeName":  "账户余额",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"locations": locations, "count": strconv.Itoa(total)})
}

func (s *AppService) CompatAdminDeposit(w http.ResponseWriter, r *http.Request) {
	params := parseAdminFormParams(r)
	address := params["address"]
	amtStr := params["amount"]
	if amtStr == "" {
		amtStr = params["usdt"]
	}
	amount, err := decimal.NewFromString(amtStr)
	if err != nil || address == "" {
		writeBizError(w, biz.ErrInvalidAmount)
		return
	}
	if _, err := s.users.Deposit(r.Context(), address, amount); err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *AppService) CompatAdminAddMoneyTwo(w http.ResponseWriter, r *http.Request) {
	params := parseAdminFormParams(r)
	address := params["address"]
	amtStr := params["usdt"]
	if amtStr == "" {
		amtStr = params["amount"]
	}
	amount, err := decimal.NewFromString(amtStr)
	if err != nil || address == "" {
		writeBizError(w, biz.ErrInvalidAmount)
		return
	}
	if _, err := s.users.SetWithdrawable(r.Context(), address, amount); err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *AppService) CompatAdminAddMoneyThree(w http.ResponseWriter, r *http.Request) {
	params := parseAdminFormParams(r)
	address := params["address"]
	amtStr := params["usdt"]
	if amtStr == "" {
		amtStr = params["amount"]
	}
	amount, err := decimal.NewFromString(amtStr)
	if err != nil || address == "" {
		writeBizError(w, biz.ErrInvalidAmount)
		return
	}
	if _, err := s.users.SetAccountBalance(r.Context(), address, amount); err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *AppService) CompatAdminVIPUpdate(w http.ResponseWriter, r *http.Request) {
	userID := formOrJSONUint64(r, "user_id")
	vipStr := formOrJSONString(r, "vip")
	vip, err := strconv.Atoi(vipStr)
	if userID == 0 || err != nil {
		writeBizError(w, biz.ErrInvalidAmount)
		return
	}
	if err := s.record.SetVIP(r.Context(), userID, vip); err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *AppService) CompatAdminVIPUnlock(w http.ResponseWriter, r *http.Request) {
	userID := formOrJSONUint64(r, "user_id")
	if userID == 0 {
		writeBizError(w, biz.ErrInvalidAmount)
		return
	}
	if err := s.record.UnlockVIP(r.Context(), userID); err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *AppService) CompatAdminLockUser(w http.ResponseWriter, r *http.Request) {
	userID := formOrJSONUint64(r, "user_id")
	lockStr := formOrJSONString(r, "lock")
	if userID == 0 || lockStr == "" {
		writeBizError(w, biz.ErrInvalidAmount)
		return
	}
	lock := lockStr == "1" || lockStr == "true"
	line := formOrJSONString(r, "one") == "1"
	var err error
	if line {
		err = s.record.SetLineLock(r.Context(), userID, lock)
	} else {
		err = s.record.SetUserLock(r.Context(), userID, lock)
	}
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *AppService) CompatAdminLockUserReward(w http.ResponseWriter, r *http.Request) {
	userID := formOrJSONUint64(r, "user_id")
	lockStr := formOrJSONString(r, "lockReward")
	if userID == 0 || lockStr == "" {
		writeBizError(w, biz.ErrInvalidAmount)
		return
	}
	locked := lockStr == "1" || lockStr == "true"
	if err := s.record.SetRewardLock(r.Context(), userID, locked); err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *AppService) CompatAdminUserRecommend(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var directs []*biz.User
	var err error
	if userIDStr := r.URL.Query().Get("userId"); userIDStr != "" {
		userID, _ := strconv.ParseUint(userIDStr, 10, 64)
		parent, err := s.record.GetUserByID(ctx, userID)
		if err != nil || parent == nil {
			writeJSON(w, http.StatusOK, map[string]any{"users": []any{}})
			return
		}
		directs, err = s.record.ListDirectReferralsByInviter(ctx, parent.ID)
	} else if address := r.URL.Query().Get("address"); address != "" {
		directs, err = s.record.ListDirectReferrals(ctx, address)
	} else {
		writeJSON(w, http.StatusOK, map[string]any{"users": []any{}})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"users": []any{}})
		return
	}
	out := make([]map[string]any, 0, len(directs))
	for _, u := range directs {
		amount, _ := s.record.SumLocationAmount(ctx, u.ID)
		activeAmt, _ := s.record.SumActiveAmount(ctx, u.ID)
		subtree, _ := s.record.SumSubtreeLocationAmount(ctx, u.ID)
		directsCount, _ := s.record.CountEffectiveDirectReferrals(ctx, u.ID)
		activated, _ := s.record.UserHasLocation(ctx, u.ID)
		out = append(out, map[string]any{
			"userId":             strconv.FormatUint(u.ID, 10),
			"address":            u.Address,
			"amount":             decStr(amount),
			"vip":                strconv.Itoa(int(u.CommunityLevel)),
			"recommendAllAmount": decStr(subtree),
			"createdAt":          u.CreatedAt.Format("2006-01-02 15:04:05"),
			"historyRecommend":   strconv.Itoa(directsCount),
			"amountUsdtCurrent":  decStr(activeAmt),
			"amountUsdtGet":      decStrFloor4(u.WithdrawableBalance),
			"balanceUsdt":        decStr(u.AccountBalance),
			"activated":          activated,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

func paginateSlice[T any](items []T, page, pageSize int) []T {
	if page < 1 {
		page = 1
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []T{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func ledgerNameCompat(entryType string) string {
	switch entryType {
	case biz.LedgerStatic:
		return "静态收益"
	case biz.LedgerGeneration:
		return "代数奖励"
	case biz.LedgerCommunityBase:
		return "社区基础奖"
	case biz.LedgerPeer:
		return "平级分红"
	case biz.LedgerExtract:
		return "提现"
	case biz.LedgerDeposit:
		return "充值"
	case biz.LedgerAdminAdjust:
		return "管理端调整可提"
	default:
		return entryType
	}
}

func ledgerReasonCompat(entryType string) string {
	switch entryType {
	case biz.LedgerStatic:
		return "static"
	case biz.LedgerGeneration:
		return "generation"
	case biz.LedgerCommunityBase:
		return "community_base"
	case biz.LedgerPeer:
		return "peer"
	case biz.LedgerExtract:
		return "extract"
	case biz.LedgerDeposit:
		return "deposit"
	case biz.LedgerAdminAdjust:
		return "admin_adjust"
	default:
		return entryType
	}
}

func ledgerEntryFromReason(reason string) string {
	switch reason {
	case "static", "location":
		return biz.LedgerStatic
	case "generation", "recommend":
		return biz.LedgerGeneration
	case "community_base", "area_two":
		return biz.LedgerCommunityBase
	case "peer", "area":
		return biz.LedgerPeer
	case "extract", "withdraw":
		return biz.LedgerExtract
	case "deposit", "buy":
		return biz.LedgerDeposit
	default:
		return ""
	}
}

func containsFold(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s == sub || len(sub) > 0 && findFold(s, sub)))
}

func findFold(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if equalFold(s[i:i+len(sub)], sub) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// mapWithdrawStatus returns status for dapp-admin (ADR 0010 raw + legacy aliases).
func mapWithdrawStatus(status string) string {
	switch status {
	case biz.WithdrawPending:
		return "pending"
	case biz.WithdrawRewarded, biz.WithdrawApproved:
		return "rewarded"
	case biz.WithdrawDoing:
		return "doing"
	case biz.WithdrawPass:
		return "pass"
	case biz.WithdrawRejected:
		return "rejected"
	case biz.WithdrawCancelled:
		return "cancelled"
	default:
		return status
	}
}

// formOrJSONUint64 reads id from x-www-form-urlencoded or JSON body.
func formOrJSONUint64(r *http.Request, key string) uint64 {
	params := adminFormParams(r)
	v := params[key]
	if v == "" {
		return 0
	}
	id, _ := strconv.ParseUint(v, 10, 64)
	return id
}

// formOrJSONString reads a string from form fields or JSON body.
func formOrJSONString(r *http.Request, key string) string {
	return adminFormParams(r)[key]
}

func adminFormParams(r *http.Request) map[string]string {
	return parseAdminFormParams(r)
}

func parseAdminFormParams(r *http.Request) map[string]string {
	out := make(map[string]string)
	_ = r.ParseForm()
	for k, v := range r.Form {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	if len(out) == 0 {
		raw, err := io.ReadAll(r.Body)
		if err == nil && len(raw) > 0 {
			// Restore body so repeated field reads still work.
			r.Body = io.NopCloser(bytes.NewReader(raw))
			var body map[string]any
			if err := json.Unmarshal(raw, &body); err == nil {
				for k, v := range body {
					switch t := v.(type) {
					case string:
						out[k] = t
					case float64:
						out[k] = strconv.FormatFloat(t, 'f', -1, 64)
					case json.Number:
						out[k] = t.String()
					default:
						out[k] = fmt.Sprint(t)
					}
				}
			}
		}
	}
	return out
}
