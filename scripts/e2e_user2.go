//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	baseURL = "http://127.0.0.1:8000"
	jwtKey  = "change-me-jwt-key"
	// 已注册的联调用户（MetaMask）
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

	userTok, err := issueUserJWT(userID, address)
	must("issue_user_jwt", err)
	ok("issue_user_jwt", "uid=2")

	adminTok, err := postJSON("/api/admin_jinniu/login", map[string]string{
		"username": "admin", "password": "admin123",
	}, "")
	must("admin_login", err)
	adminToken, _ := adminTok["token"].(string)
	if adminToken == "" {
		must("admin_login", fmt.Errorf("%v", adminTok))
	}
	ok("admin_login", "ok")

	info, err := getJSON("/api/app_server/user_info", userTok)
	must("user_info", err)
	ok("user_info", fmt.Sprintf("usdt=%v amountGet=%v recommendNum=%v", info["usdt"], info["amountGet"], info["recommendNum"]))

	// Top up account balance if needed for subscribe.
	bal := toFloat(info["usdt"])
	if bal < 100 {
		dep, err := postJSON("/api/admin_jinniu/add_money_three", map[string]any{
			"address": address, "amount": "500",
		}, adminToken)
		must("deposit", err)
		if dep["status"] != "ok" {
			must("deposit", fmt.Errorf("%v", dep))
		}
		ok("deposit", "500")
		bal = 500
	}

	buyAmt := "100"
	if bal >= 500 {
		buyAmt = "500"
	}
	buy, err := postJSON("/api/app_server/buy", map[string]any{"amount": buyAmt}, userTok)
	must("buy", err)
	if buy["status"] != "ok" {
		must("buy", fmt.Errorf("%v", buy))
	}
	ok("buy", fmt.Sprintf("id=%v amount=%s", buy["id"], buyAmt))

	recs, err := getJSON("/api/app_server/recommend_list?address="+address, userTok)
	must("recommend_list", err)
	ok("recommend_list", fmt.Sprintf("directs=%v", len(asSlice(recs["recommends"]))))

	settle, err := postJSON("/api/admin_jinniu/settle", map[string]any{"force": "1"}, adminToken)
	must("settle", err)
	if settle["status"] != "ok" {
		must("settle", fmt.Errorf("%v", settle))
	}
	ok("settle", fmt.Sprintf("settled=%v gen=%v", settle["settled_count"], settle["generation_count"]))

	info2, err := getJSON("/api/app_server/user_info", userTok)
	must("user_info_after_settle", err)
	ok("user_info_after_settle", fmt.Sprintf("usdt=%v amountGet=%v location=%v all=%v",
		info2["usdt"], info2["amountGet"], info2["location"], info2["all"]))

	wd, err := postJSON("/api/app_server/withdraw", map[string]any{"amount": "10"}, userTok)
	must("withdraw", err)
	if wd["status"] != "ok" {
		must("withdraw", fmt.Errorf("%v", wd))
	}
	ok("withdraw", fmt.Sprintf("id=%v", wd["id"]))

	pass, err := postJSON("/api/admin_jinniu/withdraw_pass", map[string]any{"id": wd["id"]}, adminToken)
	must("withdraw_pass", err)
	if pass["status"] != "ok" {
		must("withdraw_pass", fmt.Errorf("%v", pass))
	}
	ok("withdraw_pass", "approved")

	info3, err := getJSON("/api/app_server/user_info", userTok)
	must("user_info_final", err)
	ok("user_info_final", fmt.Sprintf("usdt=%v amountGet=%v", info3["usdt"], info3["amountGet"]))

	fmt.Println("E2E USER2 PASS")
}

func issueUserJWT(uid uint64, addr string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"uid":  uid,
		"addr": addr,
		"exp":  now.Add(72 * time.Hour).Unix(),
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

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		var f float64
		fmt.Sscanf(t, "%f", &f)
		return f
	default:
		return 0
	}
}

func asSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}
