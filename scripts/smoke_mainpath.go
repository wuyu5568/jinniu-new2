//go:build ignore

// S2 主路径冒烟：管理端 + 用户端（不强制日结）。
// 用法：go run scripts/smoke_mainpath.go
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

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	baseURL = "http://127.0.0.1:8000"
	// Hardhat #0 — 本地联调用户
	privHex = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	address = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"
)

type result struct {
	name   string
	status string // PASS FAIL SKIP
	detail string
}

func main() {
	var results []result
	pass := func(name, d string) { results = append(results, result{name, "PASS", d}) }
	fail := func(name, d string) { results = append(results, result{name, "FAIL", d}) }
	skip := func(name, d string) { results = append(results, result{name, "SKIP", d}) }

	// --- health (must ping MySQL) ---
	if raw, code, err := do(http.MethodGet, "/health", nil, ""); err != nil {
		fail("health", err.Error())
	} else if code != 200 {
		fail("health", fmt.Sprintf("code=%d body=%s", code, trunc(raw)))
	} else {
		var hm map[string]any
		if err := json.Unmarshal(raw, &hm); err != nil {
			fail("health", fmt.Sprintf("json: %v body=%s", err, trunc(raw)))
		} else if fmt.Sprint(hm["status"]) != "ok" || fmt.Sprint(hm["db"]) != "ok" {
			fail("health", fmt.Sprintf("want status=ok db=ok got %v", hm))
		} else {
			pass("health", "status=ok db=ok")
		}
	}

	// --- admin login ---
	adminTok := ""
	if m, code, err := doJSON(http.MethodPost, "/api/admin_jinniu/login", map[string]string{
		"username": "admin", "password": "admin123",
	}, ""); err != nil {
		fail("admin.login", err.Error())
	} else if code != 200 {
		fail("admin.login", fmt.Sprintf("code=%d body=%v", code, m))
	} else if t, _ := m["token"].(string); t == "" {
		fail("admin.login", fmt.Sprintf("no token: %v", m))
	} else {
		adminTok = t
		pass("admin.login", "token ok")
	}
	if adminTok == "" {
		printReport(results)
		os.Exit(1)
	}

	// --- admin read APIs ---
	checkObj := func(name, method, path string, body any, needKeys ...string) {
		m, code, err := doJSON(method, path, body, adminTok)
		if err != nil {
			fail(name, err.Error())
			return
		}
		if code >= 400 {
			fail(name, fmt.Sprintf("http %d: %v", code, m))
			return
		}
		missing := missingKeys(m, needKeys...)
		if len(missing) > 0 {
			fail(name, fmt.Sprintf("缺字段 %v；keys=%v", missing, keysOf(m)))
			return
		}
		pass(name, fmt.Sprintf("http %d keys ok", code))
	}

	checkObj("admin.my_auth_list", http.MethodGet, "/api/admin_jinniu/my_auth_list", nil, "auth")
	checkObj("admin.all", http.MethodGet, "/api/admin_jinniu/all", nil,
		"totalUser", "todayUser", "settle_today", "settle_today_done", "allow_force_settle",
		"payout_enabled", "payout_queue_rewarded", "payout_queue_doing")
	checkObj("admin.settle_status", http.MethodGet, "/api/admin_jinniu/settle_status", nil,
		"settle_today", "settle_today_done", "allow_force_settle")
	checkObj("admin.payout_status", http.MethodGet, "/api/admin_jinniu/payout_status", nil,
		"enabled", "key_configured", "queue_rewarded", "queue_doing", "payout_enabled", "max_required_ok")
	// deposit_replay: missing index → 400
	if m, code, err := doJSON(http.MethodPost, "/api/admin_jinniu/deposit_replay", map[string]any{}, adminTok); err != nil {
		fail("admin.deposit_replay.bad", err.Error())
	} else if code == 400 {
		pass("admin.deposit_replay.bad", "index required")
	} else {
		fail("admin.deposit_replay.bad", fmt.Sprintf("want 400 got %d %v", code, m))
	}
	// settle_status 语义：今日日期非空；done 与 today/latest 一致可读
	if m, code, err := doJSON(http.MethodGet, "/api/admin_jinniu/settle_status", nil, adminTok); err != nil {
		fail("admin.settle_status.detail", err.Error())
	} else if code >= 400 {
		fail("admin.settle_status.detail", fmt.Sprintf("http %d: %v", code, m))
	} else {
		day := fmt.Sprint(m["settle_today"])
		done := m["settle_today_done"]
		if day == "" || day == "<nil>" {
			fail("admin.settle_status.detail", fmt.Sprintf("empty settle_today: %v", m))
		} else if done == nil {
			fail("admin.settle_status.detail", "missing settle_today_done")
		} else {
			detail := fmt.Sprintf("today=%s done=%v force=%v", day, done, m["allow_force_settle"])
			if todayObj, ok := m["today"].(map[string]any); ok && done == true {
				detail += fmt.Sprintf(" static=%v", todayObj["settled_count"])
			}
			if latest, ok := m["latest"].(map[string]any); ok {
				detail += fmt.Sprintf(" latest=%v", latest["settle_date"])
			}
			pass("admin.settle_status.detail", detail)
		}
	}
	checkObj("admin.user_list", http.MethodGet, "/api/admin_jinniu/user_list?page=1", nil, "users", "count")
	checkObj("admin.buy_list", http.MethodGet, "/api/admin_jinniu/buy_list?page=1", nil, "rewards", "count")
	checkObj("admin.location_list", http.MethodGet, "/api/admin_jinniu/location_list?page=1", nil, "rewards", "count")
	checkObj("admin.withdraw_list", http.MethodGet, "/api/admin_jinniu/withdraw_list?page=1", nil, "withdraw", "count")
	checkObj("admin.reward_list", http.MethodGet, "/api/admin_jinniu/reward_list?page=1", nil, "rewards", "count")
	checkObj("admin.record_list", http.MethodGet, "/api/admin_jinniu/record_list?page=1", nil, "locations", "count")
	checkObj("admin.config", http.MethodGet, "/api/admin_jinniu/config", nil, "config")
	checkObj("admin.user_recommend", http.MethodGet, "/api/admin_jinniu/user_recommend?userId=2", nil, "users")

	// settle without force — already_settled 也算通路通
	if m, code, err := doJSON(http.MethodPost, "/api/admin_jinniu/settle", map[string]any{}, adminTok); err != nil {
		fail("admin.settle", err.Error())
	} else if code >= 400 {
		fail("admin.settle", fmt.Sprintf("http %d: %v", code, m))
	} else {
		st, _ := m["status"].(string)
		if st == "ok" || st == "already_settled" || truthy(m["skipped"]) {
			pass("admin.settle", fmt.Sprintf("status=%v skipped=%v", st, m["skipped"]))
		} else {
			fail("admin.settle", fmt.Sprintf("%v", m))
		}
	}

	// --- user eth_authorize ---
	userTok := ""
	sig, err := personalSign(address)
	if err != nil {
		fail("user.sign", err.Error())
	} else if m, code, err := doJSON(http.MethodPost, "/api/app_server/eth_authorize", map[string]string{
		"address": address, "sign": sig,
	}, ""); err != nil {
		fail("user.eth_authorize", err.Error())
	} else if code != 200 {
		fail("user.eth_authorize", fmt.Sprintf("http %d: %v", code, m))
	} else if m["status"] != "ok" {
		// 未注册需邀请码 — 记 FAIL 并尝试用创世邀请再登
		if strings.Contains(fmt.Sprint(m["status"]), "推荐") {
			sig2, _ := personalSign(address)
			m2, code2, err2 := doJSON(http.MethodPost, "/api/app_server/eth_authorize", map[string]string{
				"address": address, "sign": sig2,
				"code": "0x9ddef22beb04103ae3726807cd8af247bf2b8bf6",
			}, "")
			if err2 != nil || code2 != 200 || m2["status"] != "ok" {
				fail("user.eth_authorize", fmt.Sprintf("需邀请码且重试失败: first=%v retry=%v", m, m2))
			} else {
				userTok, _ = m2["token"].(string)
				pass("user.eth_authorize", "ok with invite code")
			}
		} else {
			fail("user.eth_authorize", fmt.Sprintf("%v", m))
		}
	} else {
		userTok, _ = m["token"].(string)
		pass("user.eth_authorize", "ok")
	}
	if userTok == "" {
		printReport(results)
		os.Exit(1)
	}

	userKeys := []string{"status", "address", "usdt", "amountGet", "level"}
	if m, code, err := doJSON(http.MethodGet, "/api/app_server/user_info", nil, userTok); err != nil {
		fail("user.user_info", err.Error())
	} else if code >= 400 {
		fail("user.user_info", fmt.Sprintf("http %d: %v", code, m))
	} else {
		missing := missingKeys(m, userKeys...)
		if len(missing) > 0 {
			fail("user.user_info", fmt.Sprintf("缺字段 %v", missing))
		} else {
			pass("user.user_info", fmt.Sprintf("usdt=%v amountGet=%v level=%v", m["usdt"], m["amountGet"], m["level"]))
		}
	}

	checkUser := func(name, method, path string, body any, needKeys ...string) map[string]any {
		m, code, err := doJSON(method, path, body, userTok)
		if err != nil {
			fail(name, err.Error())
			return nil
		}
		if code >= 400 {
			fail(name, fmt.Sprintf("http %d: %v", code, m))
			return m
		}
		if len(needKeys) > 0 {
			missing := missingKeys(m, needKeys...)
			if len(missing) > 0 {
				fail(name, fmt.Sprintf("缺字段 %v；keys=%v", missing, keysOf(m)))
				return m
			}
		}
		pass(name, fmt.Sprintf("http %d", code))
		return m
	}

	checkUser("user.subscribe_tiers", http.MethodGet, "/api/app_server/subscribe_tiers", nil, "tiers")
	checkUser("user.order_list", http.MethodGet, "/api/app_server/order_list?page=1", nil, "list", "count")
	checkUser("user.withdraw_list", http.MethodGet, "/api/app_server/withdraw_list?page=1", nil, "list", "count")
	checkUser("user.recommend_list", http.MethodGet, "/api/app_server/recommend_list?address="+address, nil, "recommends")
	checkUser("user.reward_list", http.MethodGet, "/api/app_server/reward_list?page=1", nil, "list", "count")
	checkUser("user.deposit_list", http.MethodGet, "/api/app_server/deposit_list?page=1", nil, "list", "count")

	// soft: ensure some account balance then try buy
	info, _, _ := doJSON(http.MethodGet, "/api/app_server/user_info", nil, userTok)
	usdt := toFloat(info["usdt"])
	if usdt < 100 {
		if m, code, err := doJSON(http.MethodPost, "/api/admin_jinniu/deposit", map[string]any{
			"address": address, "amount": "200",
		}, adminTok); err != nil || code >= 400 || m["status"] != "ok" {
			skip("user.buy", fmt.Sprintf("充值失败无法试认购: err=%v code=%d body=%v", err, code, m))
		} else {
			pass("admin.deposit(prep)", "200")
			usdt = 200
		}
	}
	if usdt >= 100 {
		if m, code, err := doJSON(http.MethodPost, "/api/app_server/buy", map[string]any{"amount": "100"}, userTok); err != nil {
			fail("user.buy", err.Error())
		} else if code >= 400 || m["status"] != "ok" {
			fail("user.buy", fmt.Sprintf("code=%d body=%v", code, m))
		} else {
			pass("user.buy", fmt.Sprintf("id=%v", m["id"]))
		}
	} else {
		skip("user.buy", "账户余额不足且未能充值")
	}

	info2, _, _ := doJSON(http.MethodGet, "/api/app_server/user_info", nil, userTok)
	wdBal := toFloat(info2["amountGet"])
	if wdBal < 10 {
		if m, code, err := doJSON(http.MethodPost, "/api/admin_jinniu/add_money_two", map[string]any{
			"address": address, "usdt": "20",
		}, adminTok); err != nil || code >= 400 || m["status"] != "ok" {
			skip("user.withdraw", fmt.Sprintf("设可提失败: err=%v code=%d body=%v", err, code, m))
		} else {
			pass("admin.add_money_two(prep)", "20")
			wdBal = 20
		}
	}
	if wdBal >= 10 {
		m, code, err := doJSON(http.MethodPost, "/api/app_server/withdraw", map[string]any{"amount": "10"}, userTok)
		if err != nil {
			fail("user.withdraw", err.Error())
		} else if code >= 400 || m["status"] != "ok" {
			fail("user.withdraw", fmt.Sprintf("code=%d body=%v", code, m))
		} else {
			pass("user.withdraw", fmt.Sprintf("id=%v", m["id"]))
			if id := m["id"]; id != nil {
				if cm, ccode, cerr := doJSON(http.MethodPost, "/api/app_server/withdraw_cancel", map[string]any{"id": id}, userTok); cerr != nil || ccode >= 400 || cm["status"] != "ok" {
					fail("user.withdraw_cancel", fmt.Sprintf("err=%v code=%d body=%v", cerr, ccode, cm))
				} else {
					pass("user.withdraw_cancel", "ok")
				}
			}
		}
		// second withdraw for approve → rewarded → payout_off
		m2, code2, err2 := doJSON(http.MethodPost, "/api/app_server/withdraw", map[string]any{"amount": "10"}, userTok)
		if err2 != nil {
			fail("user.withdraw2", err2.Error())
		} else if code2 >= 400 || m2["status"] != "ok" {
			fail("user.withdraw2", fmt.Sprintf("code=%d body=%v", code2, m2))
		} else {
			pass("user.withdraw2", fmt.Sprintf("id=%v", m2["id"]))
			if id := m2["id"]; id != nil {
				if pm, pcode, perr := doJSON(http.MethodPost, "/api/admin_jinniu/withdraw_pass", map[string]any{"id": id}, adminTok); perr != nil || pcode >= 400 || pm["status"] != "ok" {
					fail("admin.withdraw_pass", fmt.Sprintf("err=%v code=%d body=%v", perr, pcode, pm))
				} else {
					pass("admin.withdraw_pass", "→ rewarded")
				}
				if lm, lcode, lerr := doJSON(http.MethodGet, "/api/admin_jinniu/withdraw_list?page=1", nil, adminTok); lerr != nil || lcode >= 400 {
					fail("admin.withdraw_status", fmt.Sprintf("list err=%v code=%d", lerr, lcode))
				} else {
					found := false
					for _, it := range asSlice(lm["withdraw"]) {
						row, _ := it.(map[string]any)
						if fmt.Sprint(row["id"]) == fmt.Sprint(id) {
							found = true
							st := fmt.Sprint(row["status"])
							if st != "rewarded" && st != "approved" {
								fail("admin.withdraw_status", fmt.Sprintf("want rewarded got %s", st))
							} else {
								pass("admin.withdraw_status", st)
							}
							break
						}
					}
					if !found {
						fail("admin.withdraw_status", "id not in list")
					}
				}
				pm2, pcode2, perr2 := doJSON(http.MethodPost, "/api/admin_jinniu/withdraw_payout", map[string]any{"id": id}, adminTok)
				if perr2 != nil {
					fail("admin.withdraw_payout_off", perr2.Error())
				} else if pcode2 == 200 && pm2["status"] == "ok" {
					skip("admin.withdraw_payout_off", "payout_enabled=true（已链上或可打款）")
				} else {
					body := fmt.Sprintf("%v", pm2)
					if strings.Contains(body, "BIZ_PAYOUT_DISABLED") || strings.Contains(body, "payout disabled") || pcode2 == 403 {
						pass("admin.withdraw_payout_off", "BIZ_PAYOUT_DISABLED")
					} else {
						fail("admin.withdraw_payout_off", fmt.Sprintf("code=%d body=%v", pcode2, pm2))
					}
				}
			}
		}
	} else {
		skip("user.withdraw", "可提不足且未能设置")
	}

	printReport(results)
	for _, r := range results {
		if r.status == "FAIL" {
			os.Exit(1)
		}
	}
}

func printReport(results []result) {
	var p, f, s int
	fmt.Println()
	fmt.Println("======== S2 主路径冒烟报告 ========")
	for _, r := range results {
		fmt.Printf("%-5s %-28s %s\n", r.status, r.name, r.detail)
		switch r.status {
		case "PASS":
			p++
		case "FAIL":
			f++
		case "SKIP":
			s++
		}
	}
	fmt.Printf("-------- PASS=%d FAIL=%d SKIP=%d total=%d --------\n", p, f, s, len(results))
	fmt.Printf("time=%s\n", time.Now().Format(time.RFC3339))
}

func personalSign(msg string) (string, error) {
	key, err := crypto.HexToECDSA(privHex)
	if err != nil {
		return "", err
	}
	hash := accounts.TextHash([]byte(msg))
	sig, err := crypto.Sign(hash, key)
	if err != nil {
		return "", err
	}
	sig[64] += 27
	return hexutil.Encode(sig), nil
}

func do(method, path string, body any, bearer string) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, baseURL+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	return raw, resp.StatusCode, err
}

func doJSON(method, path string, body any, bearer string) (map[string]any, int, error) {
	raw, code, err := do(method, path, body, bearer)
	if err != nil {
		return nil, code, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, code, fmt.Errorf("json: %v body=%s", err, trunc(raw))
	}
	return out, code, nil
}

func missingKeys(m map[string]any, keys ...string) []string {
	var miss []string
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			miss = append(miss, k)
		}
	}
	return miss
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "1" || strings.EqualFold(t, "true")
	case float64:
		return t != 0
	default:
		return false
	}
}

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		var f float64
		fmt.Sscanf(strings.TrimSpace(t), "%f", &f)
		return f
	default:
		return 0
	}
}

func trunc(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

func asSlice(v any) []any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}
