//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
)

const (
	baseURL = "http://127.0.0.1:8000"
	jwtKey  = "change-me-jwt-key"
	userID  = uint64(2)
	address = "0x9ddef22beb04103ae3726807cd8af247bf2b8bf6"
)

func main() {
	must := func(step string, err error) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL [%s]: %v\n", step, err)
			os.Exit(1)
		}
	}
	ok := func(step, detail string) { fmt.Printf("OK  [%s] %s\n", step, detail) }

	// 1) health
	health, err := getRaw("/health", "")
	must("health", err)
	if !strings.Contains(string(health), "ok") && !strings.Contains(strings.ToLower(string(health)), "ok") {
		// accept {"status":"ok"} or plain ok
		var m map[string]any
		_ = json.Unmarshal(health, &m)
		if m["status"] != "ok" && string(health) != "ok" {
			must("health", fmt.Errorf("unexpected body=%s", health))
		}
	}
	ok("health", string(bytes.TrimSpace(health)))

	// 2) admin login
	adminTok, err := postJSON("/api/admin_jinniu/login", map[string]string{
		"username": "admin", "password": "admin123",
	}, "")
	must("admin_login", err)
	adminToken, _ := adminTok["token"].(string)
	if adminToken == "" {
		must("admin_login", fmt.Errorf("%v", adminTok))
	}
	ok("admin_login", "token ok")

	// 3) public subscribe tiers
	tiers, err := getJSON("/api/app_server/subscribe_tiers", "")
	must("subscribe_tiers", err)
	tierList := asSlice(tiers["tiers"])
	minSub, _ := tiers["min_subscribe_amount"].(string)
	if len(tierList) == 0 || minSub == "" {
		must("subscribe_tiers", fmt.Errorf("%v", tiers))
	}
	ok("subscribe_tiers", fmt.Sprintf("tiers=%v min=%s", tierList, minSub))

	// 4) user JWT + user_info
	userTok, err := issueUserJWT(userID, address)
	must("issue_user_jwt", err)
	info, err := getJSON("/api/app_server/user_info", userTok)
	must("user_info", err)
	for _, k := range []string{"usdt", "amountGet", "withdrawMin", "max", "min", "total", "LocationList"} {
		if _, has := info[k]; !has {
			must("user_info", fmt.Errorf("missing field %s: %v", k, info))
		}
	}
	ok("user_info", fmt.Sprintf("usdt=%v amountGet=%v withdrawMin=%v", info["usdt"], info["amountGet"], info["withdrawMin"]))

	balBefore := toFloat(info["usdt"])
	wdBefore := toFloat(info["amountGet"])

	// 5) deposit (additive) + record_list; add_money_three sets account to target
	dep, err := postJSON("/api/admin_jinniu/deposit", map[string]any{
		"address": address, "amount": "200",
	}, adminToken)
	must("deposit", err)
	if dep["status"] != "ok" {
		must("deposit", fmt.Errorf("%v", dep))
	}
	infoDep, err := getJSON("/api/app_server/user_info", userTok)
	must("user_info_after_deposit", err)
	balAfterDep := toFloat(infoDep["usdt"])
	if balAfterDep < balBefore+199 {
		must("deposit", fmt.Errorf("balance not increased: before=%v after=%v", balBefore, balAfterDep))
	}
	ok("deposit", fmt.Sprintf("usdt %v -> %v", balBefore, balAfterDep))

	targetAcc := balAfterDep + 10
	setAcc, err := postJSON("/api/admin_jinniu/add_money_three", map[string]any{
		"address": address, "amount": fmt.Sprintf("%.2f", targetAcc),
	}, adminToken)
	must("add_money_three", err)
	if setAcc["status"] != "ok" {
		must("add_money_three", fmt.Errorf("%v", setAcc))
	}
	infoSetAcc, err := getJSON("/api/app_server/user_info", userTok)
	must("user_info_after_set_account", err)
	balAfter := toFloat(infoSetAcc["usdt"])
	if absFloat(balAfter-targetAcc) > 0.01 {
		must("add_money_three", fmt.Errorf("want target=%v got=%v", targetAcc, balAfter))
	}
	ok("add_money_three", fmt.Sprintf("usdt set to %v", balAfter))

	recs, err := getJSON("/api/admin_jinniu/record_list?address="+address+"&page=1", adminToken)
	must("record_list", err)
	recLocs := asSlice(recs["locations"])
	if len(recLocs) == 0 {
		must("record_list", fmt.Errorf("empty locations: %v", recs))
	}
	ok("record_list", fmt.Sprintf("count=%v items=%d", recs["count"], len(recLocs)))

	// 6) add_money_two sets withdrawable to target
	targetWd := wdBefore + 50
	if targetWd < 0 {
		targetWd = 50
	}
	add2, err := postJSON("/api/admin_jinniu/add_money_two", map[string]any{
		"address": address, "amount": fmt.Sprintf("%.2f", targetWd),
	}, adminToken)
	must("add_money_two", err)
	if add2["status"] != "ok" {
		must("add_money_two", fmt.Errorf("%v", add2))
	}
	info2, err := getJSON("/api/app_server/user_info", userTok)
	must("user_info_after_add_money_two", err)
	wdAfterCredit := toFloat(info2["amountGet"])
	if absFloat(wdAfterCredit-targetWd) > 0.01 {
		must("add_money_two", fmt.Errorf("want target=%v got=%v", targetWd, wdAfterCredit))
	}
	ok("add_money_two", fmt.Sprintf("amountGet set to %v", wdAfterCredit))

	// 7) buy >= min + order_list has active
	buyAmt := "100"
	if toFloat(minSub) > 100 {
		buyAmt = minSub
	}
	buy, err := postJSON("/api/app_server/buy", map[string]any{"amount": buyAmt}, userTok)
	must("buy", err)
	if buy["status"] != "ok" {
		must("buy", fmt.Errorf("%v", buy))
	}
	ok("buy", fmt.Sprintf("id=%v amount=%s", buy["id"], buyAmt))

	orders, err := getJSON("/api/app_server/order_list", userTok)
	must("order_list", err)
	activeFound := false
	for _, it := range asSlice(orders["list"]) {
		m, _ := it.(map[string]any)
		if fmt.Sprint(m["status"]) == "1" {
			activeFound = true
			break
		}
	}
	if !activeFound {
		must("order_list", fmt.Errorf("no active order: %v", orders))
	}
	ok("order_list", fmt.Sprintf("count=%v active=true", orders["count"]))

	// sample rate before approve (direction may bounce at boundary; percent should move)
	buyListBefore, err := getJSON("/api/admin_jinniu/buy_list?address="+address+"&page=1", adminToken)
	must("buy_list_before", err)
	rateBefore := map[uint64]string{}
	dirBefore := map[uint64]string{}
	for _, it := range asSlice(buyListBefore["rewards"]) {
		m, _ := it.(map[string]any)
		if fmt.Sprint(m["status"]) != "active" {
			continue
		}
		id := uint64(toFloat(m["id"]))
		rateBefore[id] = fmt.Sprint(m["ratePercent"])
		dirBefore[id] = fmt.Sprint(m["rateDirection"])
	}
	ok("buy_list_before", fmt.Sprintf("active=%d sample_id_rates=%v", len(rateBefore), rateBefore))

	// 8) withdraw below min
	wdFail, err := postJSON("/api/app_server/withdraw", map[string]any{"amount": "5"}, userTok)
	must("withdraw_below_min", err)
	if wdFail["status"] != "fail" {
		must("withdraw_below_min", fmt.Errorf("expected fail: %v", wdFail))
	}
	msg := fmt.Sprint(wdFail["message"])
	if !strings.Contains(msg, "最低") {
		must("withdraw_below_min", fmt.Errorf("message missing 最低额: %v", wdFail))
	}
	ok("withdraw_below_min", msg)

	// 9) normal withdraw
	infoBeforeWd, err := getJSON("/api/app_server/user_info", userTok)
	must("user_info_before_withdraw", err)
	amountGetBeforeWd := toFloat(infoBeforeWd["amountGet"])

	wdOK, err := postJSON("/api/app_server/withdraw", map[string]any{"amount": "10"}, userTok)
	must("withdraw", err)
	if wdOK["status"] != "ok" {
		must("withdraw", fmt.Errorf("%v", wdOK))
	}
	wdID := wdOK["id"]
	ok("withdraw", fmt.Sprintf("id=%v", wdID))

	infoAfterWd, err := getJSON("/api/app_server/user_info", userTok)
	must("user_info_after_withdraw", err)
	amountGetAfterWd := toFloat(infoAfterWd["amountGet"])
	if amountGetAfterWd >= amountGetBeforeWd {
		must("withdraw_balance", fmt.Errorf("amountGet should decrease on apply: before=%v after=%v", amountGetBeforeWd, amountGetAfterWd))
	}
	ok("withdraw_balance", fmt.Sprintf("amountGet %v -> %v (deducted on apply)", amountGetBeforeWd, amountGetAfterWd))

	wdList, err := getJSON("/api/app_server/withdraw_list", userTok)
	must("withdraw_list", err)
	pendingFound := false
	for _, it := range asSlice(wdList["list"]) {
		m, _ := it.(map[string]any)
		if fmt.Sprint(m["id"]) == fmt.Sprint(wdID) && strings.EqualFold(fmt.Sprint(m["status"]), "pending") {
			pendingFound = true
			break
		}
	}
	if !pendingFound {
		must("withdraw_list", fmt.Errorf("pending id=%v not found: %v", wdID, wdList))
	}
	ok("withdraw_list", "pending ok")

	// 10) admin approve → status rewarded + active orders rateDirection may turn
	pass, err := postJSON("/api/admin_jinniu/withdraw_pass", map[string]any{"id": wdID}, adminToken)
	must("withdraw_pass", err)
	if pass["status"] != "ok" {
		must("withdraw_pass", fmt.Errorf("%v", pass))
	}
	ok("withdraw_pass", "rewarded")

	wdList2, err := getJSON("/api/app_server/withdraw_list", userTok)
	must("withdraw_list_after_pass", err)
	rewardedFound := false
	for _, it := range asSlice(wdList2["list"]) {
		m, _ := it.(map[string]any)
		st := strings.ToLower(fmt.Sprint(m["status"]))
		if fmt.Sprint(m["id"]) == fmt.Sprint(wdID) && (st == "rewarded" || st == "approved") {
			rewardedFound = true
			break
		}
	}
	if !rewardedFound {
		must("withdraw_list_after_pass", fmt.Errorf("rewarded id=%v not found: %v", wdID, wdList2))
	}
	ok("withdraw_list_after_pass", "rewarded ok")

	payout, err := postJSON("/api/admin_jinniu/withdraw_payout", map[string]any{"id": wdID}, adminToken)
	payoutBody := fmt.Sprintf("%v %v", payout, err)
	if err == nil && payout["status"] == "ok" {
		ok("withdraw_payout_off", "payout_enabled=true skip assert")
	} else if strings.Contains(payoutBody, "BIZ_PAYOUT_DISABLED") || strings.Contains(payoutBody, "payout disabled") {
		ok("withdraw_payout_off", "BIZ_PAYOUT_DISABLED")
	} else {
		must("withdraw_payout_off", fmt.Errorf("expected disabled, got body=%v err=%v", payout, err))
	}

	buyListAfter, err := getJSON("/api/admin_jinniu/buy_list?address="+address+"&page=1", adminToken)
	must("buy_list_after", err)
	rateChanged, dirChanged, sampled := 0, 0, 0
	for _, it := range asSlice(buyListAfter["rewards"]) {
		m, _ := it.(map[string]any)
		if fmt.Sprint(m["status"]) != "active" {
			continue
		}
		id := uint64(toFloat(m["id"]))
		rb, has := rateBefore[id]
		if !has {
			continue
		}
		sampled++
		if rb != fmt.Sprint(m["ratePercent"]) {
			rateChanged++
		}
		if dirBefore[id] != fmt.Sprint(m["rateDirection"]) {
			dirChanged++
		}
	}
	if sampled > 0 && rateChanged == 0 && dirChanged == 0 {
		must("rate_turn_sample", fmt.Errorf("expected ratePercent or rateDirection change after approve; sampled=%d", sampled))
	}
	ok("rate_turn_sample", fmt.Sprintf("sampled=%d rateChanged=%d dirChanged=%d", sampled, rateChanged, dirChanged))

	// 11) settle + reward lists
	settle, err := postJSON("/api/admin_jinniu/settle", map[string]any{"force": "1"}, adminToken)
	must("settle", err)
	if settle["status"] != "ok" {
		must("settle", fmt.Errorf("%v", settle))
	}
	ok("settle", fmt.Sprintf("settled=%v generation=%v community=%v peer=%v",
		settle["settled_count"], settle["generation_count"], settle["community_count"], settle["peer_count"]))

	userRewards, err := getJSON("/api/app_server/reward_list", userTok)
	must("user_reward_list", err)
	ok("user_reward_list", fmt.Sprintf("keys=%v", mapKeys(userRewards)))

	adminRewards, err := getJSON("/api/admin_jinniu/reward_list?page=1", adminToken)
	must("admin_reward_list", err)
	if asSlice(adminRewards["rewards"]) == nil && adminRewards["count"] == nil {
		must("admin_reward_list", fmt.Errorf("%v", adminRewards))
	}
	ok("admin_reward_list", fmt.Sprintf("count=%v", adminRewards["count"]))

	// 12) business config: readable names, subscribe_tiers hot reload, restore
	cfg, err := getJSON("/api/admin_jinniu/config", adminToken)
	must("config", err)
	cfgItems := asSlice(cfg["config"])
	if len(cfgItems) == 0 {
		must("config", fmt.Errorf("empty config"))
	}
	var tiersID uint64
	var tiersOrig string
	readableNames := 0
	hasSubscribeTiers := false
	for _, it := range cfgItems {
		m, _ := it.(map[string]any)
		name := fmt.Sprint(m["name"])
		key := fmt.Sprint(m["key"])
		if hasReadableCJK(name) {
			readableNames++
		}
		if key == "subscribe_tiers" {
			hasSubscribeTiers = true
			tiersID = uint64(toFloat(m["id"]))
			tiersOrig = fmt.Sprint(m["value"])
		}
	}
	if !hasSubscribeTiers {
		must("config", fmt.Errorf("missing subscribe_tiers"))
	}
	if readableNames == 0 {
		must("config", fmt.Errorf("no readable Chinese names"))
	}
	ok("config", fmt.Sprintf("items=%d readable_names=%d tiers_id=%d", len(cfgItems), readableNames, tiersID))

	tmpTiers := "100,200,300,500"
	upd, err := postJSON("/api/admin_jinniu/config_update", map[string]any{
		"id": tiersID, "value": tmpTiers,
	}, adminToken)
	must("config_update", err)
	if upd["status"] != "ok" {
		must("config_update", fmt.Errorf("%v", upd))
	}
	tiersHot, err := getJSON("/api/app_server/subscribe_tiers", "")
	must("subscribe_tiers_hot", err)
	hotList := fmt.Sprint(asSlice(tiersHot["tiers"]))
	if !strings.Contains(hotList, "200") {
		must("subscribe_tiers_hot", fmt.Errorf("expected updated tiers: %v", tiersHot))
	}
	ok("subscribe_tiers_hot", hotList)

	restore, err := postJSON("/api/admin_jinniu/config_update", map[string]any{
		"id": tiersID, "value": tiersOrig,
	}, adminToken)
	must("config_restore", err)
	if restore["status"] != "ok" {
		must("config_restore", fmt.Errorf("%v", restore))
	}
	ok("config_restore", "subscribe_tiers restored")

	// 13) dashboard
	all, err := getJSON("/api/admin_jinniu/all", adminToken)
	must("dashboard_all", err)
	for _, k := range []string{"totalUser", "buyTotal", "totalWithdraw", "balanceUsdt"} {
		if _, has := all[k]; !has {
			must("dashboard_all", fmt.Errorf("missing %s: %v", k, all))
		}
	}
	ok("dashboard_all", fmt.Sprintf("totalUser=%v buyTotal=%v", all["totalUser"], all["buyTotal"]))

	// 14) admin lists
	users, err := getJSON("/api/admin_jinniu/user_list?page=1", adminToken)
	must("user_list", err)
	if asSlice(users["users"]) == nil {
		must("user_list", fmt.Errorf("%v", users))
	}
	ok("user_list", fmt.Sprintf("count=%v", users["count"]))

	buys, err := getJSON("/api/admin_jinniu/buy_list?page=1", adminToken)
	must("buy_list", err)
	if asSlice(buys["rewards"]) == nil {
		must("buy_list", fmt.Errorf("%v", buys))
	}
	ok("buy_list", fmt.Sprintf("count=%v", buys["count"]))

	ledger, err := getJSON("/api/admin_jinniu/reward_list?page=1", adminToken)
	must("ledger_reward_list", err)
	if ledger["rewards"] == nil {
		must("ledger_reward_list", fmt.Errorf("%v", ledger))
	}
	ok("ledger_reward_list", fmt.Sprintf("count=%v", ledger["count"]))

	fmt.Println("E2E FULL PASS")
}

func issueUserJWT(uid uint64, addr string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"uid":  uid,
		"addr": addr,
		"exp":  now.Add(24 * time.Hour).Unix(),
		"iat":  now.Unix(),
		"sub":  fmt.Sprintf("%d", uid),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(jwtKey))
}

func postJSON(path string, body any, bearer string) (map[string]any, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, string(raw))
	}
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("http %d: %s", resp.StatusCode, string(raw))
	}
	return out, nil
}

func getJSON(path, bearer string) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, string(raw))
	}
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("http %d: %s", resp.StatusCode, string(raw))
	}
	return out, nil
}

func getRaw(path, bearer string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return raw, fmt.Errorf("http %d: %s", resp.StatusCode, string(raw))
	}
	return raw, nil
}

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		var f float64
		fmt.Sscanf(t, "%f", &f)
		return f
	case json.Number:
		f, _ := t.Float64()
		return f
	default:
		return 0
	}
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func asSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func hasReadableCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
