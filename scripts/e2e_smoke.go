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

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	baseURL = "http://127.0.0.1:8000"
	// Hardhat #0 — 历史联调账号（库中 id=1）；当前创世见 config genesis_address
	privHex = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	address = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"
)

func main() {
	must := func(step string, err error) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL [%s]: %v\n", step, err)
			os.Exit(1)
		}
	}
	logOK := func(step string, detail string) {
		fmt.Printf("OK  [%s] %s\n", step, detail)
	}

	// 1) admin login
	adminTok, err := postJSON("/api/admin_jinniu/login", map[string]string{
		"username": "admin", "password": "admin123",
	}, "")
	must("admin_login", err)
	adminToken, _ := adminTok["token"].(string)
	if adminToken == "" {
		must("admin_login", fmt.Errorf("empty token: %v", adminTok))
	}
	logOK("admin_login", "token ok")

	// 2) eth_authorize (genesis, no invite)
	sig, err := personalSign(address)
	must("sign", err)
	loginRes, err := postJSON("/api/app_server/eth_authorize", map[string]string{
		"address": address, "sign": sig,
	}, "")
	must("eth_authorize", err)
	userToken, _ := loginRes["token"].(string)
	if userToken == "" || loginRes["status"] != "ok" {
		must("eth_authorize", fmt.Errorf("%v", loginRes))
	}
	logOK("eth_authorize", "user token ok")

	// 3) admin deposit account balance
	dep, err := postJSON("/api/admin_jinniu/deposit", map[string]any{
		"address": address, "amount": "1000",
	}, adminToken)
	must("deposit", err)
	if dep["status"] != "ok" {
		must("deposit", fmt.Errorf("%v", dep))
	}
	logOK("deposit", "1000 account balance")

	info, err := getJSON("/api/app_server/user_info", userToken)
	must("user_info_after_deposit", err)
	logOK("user_info", fmt.Sprintf("usdt=%v amountGet=%v", info["usdt"], info["amountGet"]))

	// 4) buy / subscribe
	buy, err := postJSON("/api/app_server/buy", map[string]any{"amount": "1000"}, userToken)
	must("buy", err)
	if buy["status"] != "ok" {
		must("buy", fmt.Errorf("%v", buy))
	}
	logOK("buy", fmt.Sprintf("id=%v amount=%v", buy["id"], buy["amount"]))

	// 5) settle
	settle, err := postJSON("/api/admin_jinniu/settle", map[string]any{"force": "1"}, adminToken)
	must("settle", err)
	if settle["status"] != "ok" {
		must("settle", fmt.Errorf("%v", settle))
	}
	logOK("settle", fmt.Sprintf("settled=%v gen=%v community=%v peer=%v",
		settle["settled_count"], settle["generation_count"], settle["community_count"], settle["peer_count"]))

	info2, err := getJSON("/api/app_server/user_info", userToken)
	must("user_info_after_settle", err)
	logOK("user_info_after_settle", fmt.Sprintf("usdt=%v amountGet=%v location=%v all=%v amountGetSub=%v",
		info2["usdt"], info2["amountGet"], info2["location"], info2["all"], info2["amountGetSub"]))

	// ensure withdrawable >= 10 for withdraw path
	wdBal := toFloat(info2["amountGet"])
	if wdBal < 10 {
		_, err = postJSON("/api/admin_jinniu/add_money_two", map[string]any{
			"address": address, "usdt": "20",
		}, adminToken)
		must("add_withdrawable", err)
		logOK("add_withdrawable", "topped up to allow withdraw>=10")
	}

	// 6) withdraw
	wd, err := postJSON("/api/app_server/withdraw", map[string]any{"amount": "10"}, userToken)
	must("withdraw", err)
	if wd["status"] != "ok" {
		must("withdraw", fmt.Errorf("%v", wd))
	}
	wdID := wd["id"]
	logOK("withdraw", fmt.Sprintf("id=%v", wdID))

	// 7) admin approve
	pass, err := postJSON("/api/admin_jinniu/withdraw_pass", map[string]any{"id": wdID}, adminToken)
	must("withdraw_pass", err)
	if pass["status"] != "ok" {
		must("withdraw_pass", fmt.Errorf("%v", pass))
	}
	logOK("withdraw_pass", "approved")

	info3, err := getJSON("/api/app_server/user_info", userToken)
	must("user_info_final", err)
	logOK("user_info_final", fmt.Sprintf("usdt=%v amountGet=%v all=%v", info3["usdt"], info3["amountGet"], info3["all"]))

	rewards, err := getJSON("/api/app_server/reward_list?page=1&reqType=2", userToken)
	must("reward_list", err)
	logOK("reward_list", fmt.Sprintf("count=%v", rewards["count"]))

	fmt.Println("E2E PASS")
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

func getJSON(path string, bearer string) (map[string]any, error) {
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
