//go:build ignore

// W1 管理端写操作沙箱冒烟：VIP / 锁定 / 停分红，测完改回。
// 用法：go run scripts/smoke_admin_write.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	baseURL = "http://127.0.0.1:8000"
	address = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266" // Hardhat #0
)

type result struct {
	name   string
	status string
	detail string
}

func main() {
	var results []result
	pass := func(n, d string) { results = append(results, result{n, "PASS", d}) }
	fail := func(n, d string) { results = append(results, result{n, "FAIL", d}) }

	adminTok, err := adminLogin()
	if err != nil {
		fail("admin.login", err.Error())
		printReport(results)
		os.Exit(1)
	}
	pass("admin.login", "ok")

	u, err := findUser(adminTok, address)
	if err != nil {
		fail("admin.user_list.find", err.Error())
		printReport(results)
		os.Exit(1)
	}
	userID := str(u["userId"])
	origVIP := str(u["vip"])
	origLock := str(u["lock"])
	origLockReward := str(u["lockReward"])
	pass("snapshot", fmt.Sprintf("userId=%s vip=%s lock=%s lockReward=%s", userID, origVIP, origLock, origLockReward))

	// --- VIP：改到不同档再改回 ---
	targetVIP := "3"
	if origVIP == "3" {
		targetVIP = "4"
	}
	if err := postOK(adminTok, "/api/admin_jinniu/vip_update", map[string]any{
		"user_id": userID, "vip": targetVIP,
	}); err != nil {
		fail("admin.vip_update.set", err.Error())
	} else {
		u2, err := findUser(adminTok, address)
		if err != nil {
			fail("admin.vip_update.verify", err.Error())
		} else if str(u2["vip"]) != targetVIP {
			fail("admin.vip_update.verify", fmt.Sprintf("want vip=%s got=%s", targetVIP, u2["vip"]))
		} else {
			pass("admin.vip_update.set", "vip="+targetVIP)
		}
	}
	if err := postOK(adminTok, "/api/admin_jinniu/vip_update", map[string]any{
		"user_id": userID, "vip": origVIP,
	}); err != nil {
		fail("admin.vip_update.restore", err.Error())
	} else {
		u3, _ := findUser(adminTok, address)
		if str(u3["vip"]) != origVIP {
			fail("admin.vip_update.restore", fmt.Sprintf("want=%s got=%s", origVIP, u3["vip"]))
		} else {
			pass("admin.vip_update.restore", "vip="+origVIP)
		}
	}

	// --- 用户锁定（非整线）：翻转到对立态再改回 ---
	flipLock := "1"
	if origLock == "1" {
		flipLock = "0"
	}
	if err := postOK(adminTok, "/api/admin_jinniu/lock_user", map[string]any{
		"user_id": userID, "lock": flipLock,
	}); err != nil {
		fail("admin.lock_user.set", err.Error())
	} else {
		u2, err := findUser(adminTok, address)
		if err != nil {
			fail("admin.lock_user.verify", err.Error())
		} else if str(u2["lock"]) != flipLock {
			fail("admin.lock_user.verify", fmt.Sprintf("want lock=%s got=%s", flipLock, u2["lock"]))
		} else {
			pass("admin.lock_user.set", "lock="+flipLock)
		}
	}
	if err := postOK(adminTok, "/api/admin_jinniu/lock_user", map[string]any{
		"user_id": userID, "lock": origLock,
	}); err != nil {
		fail("admin.lock_user.restore", err.Error())
	} else {
		u3, _ := findUser(adminTok, address)
		if str(u3["lock"]) != origLock {
			fail("admin.lock_user.restore", fmt.Sprintf("want=%s got=%s", origLock, u3["lock"]))
		} else {
			pass("admin.lock_user.restore", "lock="+origLock)
		}
	}

	// --- 停分红 ---
	flipReward := "1"
	if origLockReward == "1" {
		flipReward = "0"
	}
	if err := postOK(adminTok, "/api/admin_jinniu/lock_user_reward", map[string]any{
		"user_id": userID, "lockReward": flipReward,
	}); err != nil {
		fail("admin.lock_user_reward.set", err.Error())
	} else {
		u2, err := findUser(adminTok, address)
		if err != nil {
			fail("admin.lock_user_reward.verify", err.Error())
		} else if str(u2["lockReward"]) != flipReward {
			fail("admin.lock_user_reward.verify", fmt.Sprintf("want=%s got=%s", flipReward, u2["lockReward"]))
		} else {
			pass("admin.lock_user_reward.set", "lockReward="+flipReward)
		}
	}
	if err := postOK(adminTok, "/api/admin_jinniu/lock_user_reward", map[string]any{
		"user_id": userID, "lockReward": origLockReward,
	}); err != nil {
		fail("admin.lock_user_reward.restore", err.Error())
	} else {
		u3, _ := findUser(adminTok, address)
		if str(u3["lockReward"]) != origLockReward {
			fail("admin.lock_user_reward.restore", fmt.Sprintf("want=%s got=%s", origLockReward, u3["lockReward"]))
		} else {
			pass("admin.lock_user_reward.restore", "lockReward="+origLockReward)
		}
	}

	printReport(results)
	for _, r := range results {
		if r.status == "FAIL" {
			os.Exit(1)
		}
	}
}

func printReport(results []result) {
	var p, f int
	fmt.Println()
	fmt.Println("======== W1 管理端写操作冒烟 ========")
	for _, r := range results {
		fmt.Printf("%-5s %-32s %s\n", r.status, r.name, r.detail)
		if r.status == "PASS" {
			p++
		} else {
			f++
		}
	}
	fmt.Printf("-------- PASS=%d FAIL=%d --------\n", p, f)
	fmt.Printf("time=%s\n", time.Now().Format(time.RFC3339))
}

func adminLogin() (string, error) {
	m, code, err := doJSON(http.MethodPost, "/api/admin_jinniu/login", map[string]string{
		"username": "admin", "password": "admin123",
	}, "")
	if err != nil {
		return "", err
	}
	if code != 200 {
		return "", fmt.Errorf("http %d: %v", code, m)
	}
	t, _ := m["token"].(string)
	if t == "" {
		return "", fmt.Errorf("empty token: %v", m)
	}
	return t, nil
}

func findUser(token, addr string) (map[string]any, error) {
	m, code, err := doJSON(http.MethodGet, "/api/admin_jinniu/user_list?page=1&address="+addr, nil, token)
	if err != nil {
		return nil, err
	}
	if code >= 400 {
		return nil, fmt.Errorf("http %d: %v", code, m)
	}
	arr, _ := m["users"].([]any)
	if len(arr) == 0 {
		return nil, fmt.Errorf("user not found: %s", addr)
	}
	u, ok := arr[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("bad user row: %v", arr[0])
	}
	return u, nil
}

func postOK(token, path string, body map[string]any) error {
	m, code, err := doJSON(http.MethodPost, path, body, token)
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("http %d: %v", code, m)
	}
	if st, ok := m["status"].(string); ok && st != "ok" {
		return fmt.Errorf("status=%s body=%v", st, m)
	}
	return nil
}

func doJSON(method, path string, body any, bearer string) (map[string]any, int, error) {
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
	if err != nil {
		return nil, resp.StatusCode, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("json: %v body=%s", err, string(raw))
	}
	return out, resp.StatusCode, nil
}

func str(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
