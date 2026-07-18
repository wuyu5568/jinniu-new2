//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
)

// Demo seed: write invite tree + location amounts only.
// community_level / community_volume are computed with the same formulas as settleCommunity
// (SubtreeVolume + SmallAreaVolume + LevelFromVolume). Never hand-set V 级.

type demoUser struct {
	Label     string
	Address   string
	Parent    string // label, or "" → hang under uid=2
	Account   string
	Withdraw  string
	ExpectVIP uint8 // expected level after refresh; validated
	Locked    bool
	RewLock   bool
	Locations []locSeed
	Withdraws []wdSeed
	Ledgers   []ledSeed
}

type locSeed struct {
	Amount, Multiplier, ExitTarget, Accumulated, RatePercent, Direction, Status string
}

type wdSeed struct {
	Amount, Fee, Credited, Status, Remark string
}

type ledSeed struct {
	EntryType, Amount, BalanceKind, Remark string
	OrderIdx                               int
}

// V1–V9 小区业绩门槛（与 DefaultBusinessParams / schema 一致）
var tierMin = []string{
	"0",        // 0 unused
	"5000",     // V1
	"20000",    // V2
	"80000",    // V3
	"250000",   // V4
	"500000",   // V5
	"1500000",  // V6
	"5000000",  // V7
	"10000000", // V8
	"20000000", // V9
}

func main() {
	must := func(step string, err error) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL [%s]: %v\n", step, err)
			os.Exit(1)
		}
	}

	dsn, err := loadDSN()
	must("load_dsn", err)
	db, err := sql.Open("mysql", dsn)
	must("open_db", err)
	defer db.Close()
	must("ping", db.Ping())

	var rootID uint64
	err = db.QueryRow(`SELECT id FROM users WHERE address = ?`, "0x9ddef22beb04103ae3726807cd8af247bf2b8bf6").Scan(&rootID)
	must("find_root_uid2", err)
	rootPath, err := getPath(db, rootID)
	must("root_path", err)
	if rootPath == "" {
		rootPath = strconv.FormatUint(rootID, 10)
	}

	demos := buildDemos()
	byLabel := map[string]*demoUser{}
	for i := range demos {
		byLabel[demos[i].Label] = &demos[i]
	}
	order := seedOrder(demos)

	inserted, updated := 0, 0
	ids := map[string]uint64{}

	tx, err := db.Begin()
	must("begin", err)
	defer func() { _ = tx.Rollback() }()

	for _, label := range order {
		d := byLabel[label]
		parentID := rootID
		parentPath := rootPath
		if d.Parent != "" {
			pid, ok := ids[d.Parent]
			if !ok {
				must("parent_"+d.Parent, fmt.Errorf("parent not seeded yet"))
			}
			parentID = pid
			parentPath, err = getPathTx(tx, parentID)
			must("parent_path_"+d.Parent, err)
		}

		var existingID uint64
		err = tx.QueryRow(`SELECT id FROM users WHERE address = ?`, d.Address).Scan(&existingID)
		now := time.Now()
		var disabled any
		if d.Locked {
			disabled = now
		} else {
			disabled = nil
		}
		rew := 0
		if d.RewLock {
			rew = 1
		}
		// 等级一律先写 0，稍后按认购业绩重算
		if err == sql.ErrNoRows {
			res, err := tx.Exec(`
				INSERT INTO users (address, inviter_id, account_balance, withdrawable_balance,
					community_level, community_volume, disabled_at, reward_locked, created_at, updated_at)
				VALUES (?, ?, ?, ?, 0, 0, ?, ?, ?, ?)`,
				d.Address, parentID, d.Account, d.Withdraw, disabled, rew, now, now)
			must("insert_"+d.Label, err)
			id, _ := res.LastInsertId()
			existingID = uint64(id)
			inserted++
		} else if err != nil {
			must("select_"+d.Label, err)
		} else {
			_, err = tx.Exec(`
				UPDATE users SET inviter_id=?, account_balance=?, withdrawable_balance=?,
					community_level=0, community_volume=0, disabled_at=?, reward_locked=?, updated_at=?
				WHERE id=?`,
				parentID, d.Account, d.Withdraw, disabled, rew, now, existingID)
			must("update_"+d.Label, err)
			updated++
		}
		ids[d.Label] = existingID

		path := parentPath + "," + strconv.FormatUint(existingID, 10)
		must("save_path_"+d.Label, upsertPath(tx, existingID, path))
		must("clear_rel_"+d.Label, clearRelated(tx, existingID))

		var locIDs []uint64
		for _, loc := range d.Locations {
			res, err := tx.Exec(`
				INSERT INTO locations (user_id, amount, multiplier, exit_target, accumulated, status, rate_percent, rate_direction, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				existingID, loc.Amount, loc.Multiplier, loc.ExitTarget, loc.Accumulated, loc.Status, loc.RatePercent, loc.Direction, now, now)
			must("loc_"+d.Label, err)
			lid, _ := res.LastInsertId()
			locIDs = append(locIDs, uint64(lid))
		}

		for _, w := range d.Withdraws {
			var reviewed any
			if w.Status == "rewarded" || w.Status == "approved" || w.Status == "rejected" || w.Status == "pass" {
				reviewed = now
			}
			_, err = tx.Exec(`
				INSERT INTO withdraws (user_id, amount, fee_amount, credited_amount, order_ids_json, status, remark, reviewed_at, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				existingID, w.Amount, w.Fee, w.Credited, "[]", w.Status, w.Remark, reviewed, now, now)
			must("wd_"+d.Label, err)
		}

		for _, l := range d.Ledgers {
			var orderID any
			if l.OrderIdx > 0 && l.OrderIdx <= len(locIDs) {
				orderID = locIDs[l.OrderIdx-1]
			}
			_, err = tx.Exec(`
				INSERT INTO ledger_entries (user_id, order_id, entry_type, amount, balance_kind, remark, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				existingID, orderID, l.EntryType, l.Amount, l.BalanceKind, l.Remark, now)
			must("led_"+d.Label, err)
		}

		fmt.Printf("OK  [%s] id=%d addr=%s expectV=%d\n", d.Label, existingID, d.Address, d.ExpectVIP)
	}

	must("commit", tx.Commit())
	fmt.Printf("SEED rows inserted=%d updated=%d total=%d\n", inserted, updated, len(order))

	must("cleanup_orphans", cleanupOrphanDemos(db, demos))
	must("refresh_community", refreshCommunityLevels(db))
	must("validate_expect_vip", validateExpectVIP(db, demos, ids))
	must("validate_peer_topo", validatePeerTopo(db, ids))
	fmt.Println("SEED DEMO USERS DONE (levels from subscribe volume)")
}

func buildDemos() []demoUser {
	demos := []demoUser{
		{Label: "zero", Address: addr(1), Account: "0", Withdraw: "0"},
		{
			Label: "account_only", Address: addr(2),
			Account: "500.00000000", Withdraw: "0",
			Ledgers: []ledSeed{{EntryType: "deposit", Amount: "500", BalanceKind: "account", Remark: "admin deposit"}},
		},
		{
			Label: "active_buy", Address: addr(3),
			Account: "0", Withdraw: "5.00000000",
			Locations: []locSeed{locBuy("100")},
			Ledgers: []ledSeed{
				{EntryType: "deposit", Amount: "100", BalanceKind: "account", Remark: "admin deposit"},
				{EntryType: "static", Amount: "5", BalanceKind: "withdrawable", Remark: "demo static", OrderIdx: 1},
			},
		},
		{
			Label: "exited_only", Address: addr(4),
			Account: "0", Withdraw: "80.00000000",
			Locations: []locSeed{locExited("100", "200")},
			Ledgers: []ledSeed{
				{EntryType: "static", Amount: "80", BalanceKind: "withdrawable", Remark: "demo exited static", OrderIdx: 1},
			},
		},
		{
			Label: "rate_down", Address: addr(5),
			Account: "50.00000000", Withdraw: "12.00000000",
			Locations: []locSeed{locBuyDir("500", "down")},
		},
		// V1：两直推腿各 ≥5000 认购
		{
			Label: "high_wd", Address: addr(6), ExpectVIP: 1,
			Account: "0", Withdraw: "888.00000000",
			Locations: []locSeed{locBuy("500")},
			Ledgers: []ledSeed{
				{EntryType: "static", Amount: "400", BalanceKind: "withdrawable", Remark: "demo static", OrderIdx: 1},
				{EntryType: "community_base", Amount: "100", BalanceKind: "withdrawable", Remark: "demo community"},
			},
		},
		{Label: "high_wd_c1", Address: addr(70), Parent: "high_wd", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("5000")}},
		{Label: "high_wd_c2", Address: addr(71), Parent: "high_wd", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("5000")}},
		{
			Label: "pending_wd", Address: addr(7),
			Account: "0", Withdraw: "20.00000000",
			Locations: []locSeed{locBuy("100")},
			Withdraws: []wdSeed{{Amount: "50", Fee: "3", Credited: "47", Status: "pending"}},
		},
		{
			Label: "rewarded_wd", Address: addr(8),
			Account: "0", Withdraw: "40.00000000",
			Locations: []locSeed{locBuyDir("100", "down")},
			Withdraws: []wdSeed{{Amount: "30", Fee: "1.8", Credited: "28.2", Status: "rewarded"}},
			Ledgers: []ledSeed{
				{EntryType: "extract", Amount: "-30", BalanceKind: "withdrawable", Remark: "withdraw rewarded fee=1.8"},
			},
		},
		{Label: "locked", Address: addr(9), Account: "100.00000000", Withdraw: "10.00000000", Locked: true},
		{
			Label: "reward_lock", Address: addr(10), RewLock: true,
			Account: "0", Withdraw: "15.00000000",
			Locations: []locSeed{locBuy("100")},
		},
		// V3：两直推腿各 ≥80000
		{
			Label: "vip3", Address: addr(11), ExpectVIP: 3,
			Account: "200.00000000", Withdraw: "150.00000000",
			Locations: []locSeed{locBuy("1000")},
			Ledgers: []ledSeed{
				{EntryType: "deposit", Amount: "1200", BalanceKind: "account", Remark: "admin deposit"},
				{EntryType: "community_base", Amount: "100", BalanceKind: "withdrawable", Remark: "demo community"},
			},
		},
		{Label: "vip3_c1", Address: addr(13), Parent: "vip3", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("80000")}},
		{Label: "vip3_c2", Address: addr(14), Parent: "vip3", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("80000")}},
		{Label: "multi_ref", Address: addr(12), Account: "0", Withdraw: "0"},
		{Label: "multi_c1", Address: addr(15), Parent: "multi_ref", Account: "0", Withdraw: "0"},
		{Label: "multi_c2", Address: addr(16), Parent: "multi_ref", Account: "0", Withdraw: "0"},
	}
	demos = append(demos, levelLadderDemos()...)
	demos = append(demos, peerRelationDemos()...)
	demos = append(demos, wideRecommendDemos()...)
	return demos
}

// Deep V9→V0 chain. Each Vn has chain-child + fill leg; fill buy = tierMin[n];
// chain personal padded so subtree(chain) >= tierMin[n] (min of two legs = threshold).
func levelLadderDemos() []demoUser {
	// personal pad so after children, subtree(lv_vn) >= next threshold for parent
	// Bottom-up:
	// P0=5000; F1=5000 → V1; subtree1 = P1+5000+5000, need >=20000 → P1=10000
	// F2=20000; subtree2 = P2+20000+20000 >=80000 → P2=40000
	// F3=80000; subtree3 = P3+80000+80000 >=250000 → P3=90000
	// F4=250000; subtree4 = P4+250000+250000 >=500000 → P4=0
	// F5=500000; subtree5 = P5+500000+500000 >=1500000 → P5=500000
	// F6=1500000; subtree6 = P6+1.5M+1.5M >=5M → P6=2M
	// F7=5M; subtree7 = P7+5M+5M >=10M → P7=0
	// F8=10M; subtree8 = P8+10M+10M >=20M → P8=0
	// F9=20M
	personals := map[int]string{
		0: "5000",
		1: "10000",
		2: "40000",
		3: "90000",
		4: "0",
		5: "500000",
		6: "2000000",
		7: "0",
		8: "0",
		9: "0",
	}
	out := []demoUser{}
	for n := 9; n >= 0; n-- {
		parent := ""
		if n < 9 {
			parent = fmt.Sprintf("lv_v%d", n+1)
		}
		u := demoUser{
			Label: fmt.Sprintf("lv_v%d", n), Address: addr(17 + (9 - n)), Parent: parent,
			Account: "0", Withdraw: "20.00000000", ExpectVIP: uint8(n),
		}
		if p := personals[n]; p != "0" {
			u.Locations = []locSeed{locBuy(p)}
		} else if n == 0 {
			u.Locations = []locSeed{locBuy("5000")}
		} else {
			u.Locations = []locSeed{locBuy("100")} // tiny self buy; legs carry volume
		}
		out = append(out, u)
		if n >= 1 {
			out = append(out, demoUser{
				Label: fmt.Sprintf("lv_v%d_fill", n), Address: addr(80 + n), Parent: fmt.Sprintf("lv_v%d", n),
				Account: "0", Withdraw: "0",
				Locations: []locSeed{locBuy(tierMin[n])},
			})
		}
	}
	return out
}

func peerRelationDemos() []demoUser {
	// 平级条件：来源须在领取人小区（非最大腿）。故 fill 腿业绩必须 > 链腿（含 src）子树。
	// dual V3: upper_fill(170000) > lower_sub(100+80000+80000=160100)
	return []demoUser{
		{
			Label: "peer3_hub", Address: addr(30), ExpectVIP: 5,
			Account: "0", Withdraw: "100.00000000",
			Locations: []locSeed{locBuy("100")},
		},
		{Label: "peer3_bigleg", Address: addr(31), Parent: "peer3_hub", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("600000")}},
		{
			Label: "peer3_upper", Address: addr(32), Parent: "peer3_hub", ExpectVIP: 3,
			Account: "0", Withdraw: "60.00000000",
			Locations: []locSeed{locBuy("340000")},
		},
		{Label: "peer3_upper_fill", Address: addr(33), Parent: "peer3_upper", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("170000")}},
		{
			Label: "peer3_lower", Address: addr(34), Parent: "peer3_upper", ExpectVIP: 3,
			Account: "0", Withdraw: "30.00000000",
			Locations: []locSeed{locBuy("100")},
		},
		{Label: "peer3_lower_fill", Address: addr(35), Parent: "peer3_lower", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("80000")}},
		{Label: "peer3_src", Address: addr(36), Parent: "peer3_lower", Account: "0", Withdraw: "8.00000000",
			Locations: []locSeed{locBuy("80000")}},

		// dual V5：V4 两腿门槛之和恰达 V5，父节点无法保持 V4；改用 V5 同级才能打出平级
		// upper_fill(1010000) > lower_sub(100+500000+500000=1000100)
		{Label: "peer4_hub", Address: addr(40), ExpectVIP: 6, Account: "0", Withdraw: "80.00000000",
			Locations: []locSeed{locBuy("100")}},
		{Label: "peer4_bigleg", Address: addr(41), Parent: "peer4_hub", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("2000000")}},
		{Label: "peer4_upper", Address: addr(42), Parent: "peer4_hub", ExpectVIP: 5, Account: "0", Withdraw: "55.00000000",
			Locations: []locSeed{locBuy("1000000")},
		},
		{Label: "peer4_upper_fill", Address: addr(43), Parent: "peer4_upper", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("1010000")}},
		{Label: "peer4_lower", Address: addr(44), Parent: "peer4_upper", ExpectVIP: 5, Account: "0", Withdraw: "25.00000000",
			Locations: []locSeed{locBuy("100")}},
		{Label: "peer4_lower_fill", Address: addr(45), Parent: "peer4_lower", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("500000")}},
		{Label: "peer4_src", Address: addr(46), Parent: "peer4_lower", Account: "0", Withdraw: "9.00000000",
			Locations: []locSeed{locBuy("500000")}},

		// triple：b 为同级平级领取人；a 因含 b_fill 子树会升到 V4（水位节点），不要求 a 收 peer
		{Label: "peer3x_hub", Address: addr(50), ExpectVIP: 7, Account: "0", Withdraw: "70.00000000",
			Locations: []locSeed{locBuy("100")}},
		{Label: "peer3x_big", Address: addr(51), Parent: "peer3x_hub", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("6000000")}},
		{Label: "peer3x_a", Address: addr(52), Parent: "peer3x_hub", ExpectVIP: 4, Account: "0", Withdraw: "40.00000000",
			Locations: []locSeed{locBuy("5000000")},
		},
		{Label: "peer3x_a_fill", Address: addr(53), Parent: "peer3x_a", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("340000")}},
		{Label: "peer3x_b", Address: addr(54), Parent: "peer3x_a", ExpectVIP: 3, Account: "0", Withdraw: "35.00000000",
			Locations: []locSeed{locBuy("100")},
		},
		{Label: "peer3x_b_fill", Address: addr(55), Parent: "peer3x_b", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("170000")}},
		{Label: "peer3x_c", Address: addr(56), Parent: "peer3x_b", ExpectVIP: 3, Account: "0", Withdraw: "22.00000000",
			Locations: []locSeed{locBuy("100")}},
		{Label: "peer3x_c_fill", Address: addr(57), Parent: "peer3x_c", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("80000")}},
		{Label: "peer3x_src", Address: addr(58), Parent: "peer3x_c", Account: "0", Withdraw: "11.00000000",
			Locations: []locSeed{locBuy("80000")}},
	}
}

func wideRecommendDemos() []demoUser {
	return []demoUser{
		{Label: "wide_hub", Address: addr(60), ExpectVIP: 4, Account: "100.00000000", Withdraw: "50.00000000",
			Locations: []locSeed{locBuy("100")}},
		{Label: "wide_v0", Address: addr(61), Parent: "wide_hub", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("100000")}},
		// V1：两腿各 5000
		{Label: "wide_v1", Address: addr(62), Parent: "wide_hub", ExpectVIP: 1, Account: "0", Withdraw: "10.00000000",
			Locations: []locSeed{locBuy("100")}},
		{Label: "wide_v1_a", Address: addr(90), Parent: "wide_v1", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("5000")}},
		{Label: "wide_v1_b", Address: addr(91), Parent: "wide_v1", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("5000")}},
		// 两腿各 100000 → 小区 100000 → V3（严格按门槛，不再伪称 V2）
		{Label: "wide_v2", Address: addr(63), Parent: "wide_hub", ExpectVIP: 3, Account: "0", Withdraw: "15.00000000",
			Locations: []locSeed{locBuy("100")}},
		{Label: "wide_v2_a", Address: addr(92), Parent: "wide_v2", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("100000")}},
		{Label: "wide_v2_b", Address: addr(93), Parent: "wide_v2", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("100000")}},
		// 另建真正的 V2 节点（两腿恰为 20000）
		{Label: "wide_real_v2", Address: addr(96), Parent: "wide_hub", ExpectVIP: 2, Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("100")}},
		{Label: "wide_real_v2_a", Address: addr(97), Parent: "wide_real_v2", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("20000")}},
		{Label: "wide_real_v2_b", Address: addr(98), Parent: "wide_real_v2", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("20000")}},
		// V3：两腿各 ≥80000
		{Label: "wide_v3", Address: addr(64), Parent: "wide_hub", ExpectVIP: 3, Account: "0", Withdraw: "25.00000000",
			Locations: []locSeed{locBuy("100")},
		},
		{Label: "wide_v3_fill", Address: addr(94), Parent: "wide_v3", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("80000")}},
		{Label: "wide_v3_child", Address: addr(65), Parent: "wide_v3", ExpectVIP: 1, Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("70000")}},
		{Label: "wide_v3_child_a", Address: addr(95), Parent: "wide_v3_child", Account: "0", Withdraw: "0",
			Locations: []locSeed{locBuy("5000")}},
		{Label: "wide_v3_gchild", Address: addr(66), Parent: "wide_v3_child", Account: "0", Withdraw: "3.00000000",
			Locations: []locSeed{locBuy("5000")}},
	}
}

// refreshCommunityLevels mirrors biz.RecordUseCase.settleCommunity level update.
func refreshCommunityLevels(db *sql.DB) error {
	type urow struct {
		ID        uint64
		InviterID sql.NullInt64
	}
	rows, err := db.Query(`SELECT id, inviter_id FROM users`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var users []urow
	children := map[uint64][]uint64{}
	for rows.Next() {
		var u urow
		if err := rows.Scan(&u.ID, &u.InviterID); err != nil {
			return err
		}
		users = append(users, u)
		if u.InviterID.Valid {
			pid := uint64(u.InviterID.Int64)
			children[pid] = append(children[pid], u.ID)
		}
	}

	personal := map[uint64]decimal.Decimal{}
	prows, err := db.Query(`SELECT user_id, COALESCE(SUM(amount),0) FROM locations GROUP BY user_id`)
	if err != nil {
		return err
	}
	defer prows.Close()
	for prows.Next() {
		var uid uint64
		var sum string
		if err := prows.Scan(&uid, &sum); err != nil {
			return err
		}
		personal[uid], _ = decimal.NewFromString(sum)
	}
	for _, u := range users {
		if _, ok := personal[u.ID]; !ok {
			personal[u.ID] = decimal.Zero
		}
	}

	for _, u := range users {
		legs := children[u.ID]
		legVols := make([]decimal.Decimal, len(legs))
		for i, leg := range legs {
			legVols[i] = subtreeVolume(leg, children, personal)
		}
		vol := smallAreaVolume(legVols)
		lv := levelFromVolume(vol)
		if _, err := db.Exec(`UPDATE users SET community_level=?, community_volume=?, updated_at=? WHERE id=?`,
			lv, vol.StringFixed(8), time.Now(), u.ID); err != nil {
			return err
		}
		if lv > 0 {
			fmt.Printf("OK  [level] id=%d V%d small_area=%s directs=%d\n", u.ID, lv, vol.String(), len(legs))
		}
	}
	return nil
}

func subtreeVolume(root uint64, children map[uint64][]uint64, personal map[uint64]decimal.Decimal) decimal.Decimal {
	sum := personal[root]
	stack := append([]uint64(nil), children[root]...)
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		sum = sum.Add(personal[n])
		stack = append(stack, children[n]...)
	}
	return sum
}

func smallAreaVolume(legVolumes []decimal.Decimal) decimal.Decimal {
	if len(legVolumes) <= 1 {
		return decimal.Zero
	}
	maxIdx := 0
	for i := 1; i < len(legVolumes); i++ {
		if legVolumes[i].GreaterThan(legVolumes[maxIdx]) {
			maxIdx = i
		}
	}
	sum := decimal.Zero
	for i, v := range legVolumes {
		if i == maxIdx {
			continue
		}
		sum = sum.Add(v)
	}
	return sum
}

func levelFromVolume(smallArea decimal.Decimal) int {
	if !smallArea.IsPositive() {
		return 0
	}
	best := 0
	for lv := 1; lv <= 9; lv++ {
		minVol := decimal.RequireFromString(tierMin[lv])
		if smallArea.GreaterThanOrEqual(minVol) && lv > best {
			best = lv
		}
	}
	return best
}

func validateExpectVIP(db *sql.DB, demos []demoUser, ids map[string]uint64) error {
	bad := 0
	for _, d := range demos {
		uid := ids[d.Label]
		var lv int
		var vol string
		var directs int
		if err := db.QueryRow(`SELECT community_level, community_volume FROM users WHERE id=?`, uid).Scan(&lv, &vol); err != nil {
			return err
		}
		_ = db.QueryRow(`SELECT COUNT(*) FROM users WHERE inviter_id=?`, uid).Scan(&directs)
		if int(d.ExpectVIP) != lv {
			fmt.Printf("FAIL [expect] %s expectV=%d gotV=%d small_area=%s directs=%d\n", d.Label, d.ExpectVIP, lv, vol, directs)
			bad++
		} else {
			fmt.Printf("OK  [expect] %s V%d small_area=%s directs=%d\n", d.Label, lv, vol, directs)
		}
	}
	if bad > 0 {
		return fmt.Errorf("%d users level mismatch vs ExpectVIP", bad)
	}
	return nil
}

// validatePeerTopo checks 小区同级平级：claimant 对 peerSrc 本人静态应产生平级。
func validatePeerTopo(db *sql.DB, ids map[string]uint64) error {
	type expect struct {
		claimant string
		peerSrc  string
	}
	cases := []expect{
		{claimant: "peer3_upper", peerSrc: "peer3_lower"},
		{claimant: "peer4_upper", peerSrc: "peer4_lower"},
		{claimant: "peer3x_b", peerSrc: "peer3x_c"},
	}

	parent := map[uint64]uint64{}
	children := map[uint64][]uint64{}
	levels := map[uint64]int{}
	personal := map[uint64]decimal.Decimal{}

	rows, err := db.Query(`SELECT id, inviter_id, community_level FROM users`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id uint64
		var inv sql.NullInt64
		var lv int
		if err := rows.Scan(&id, &inv, &lv); err != nil {
			return err
		}
		levels[id] = lv
		if inv.Valid {
			pid := uint64(inv.Int64)
			parent[id] = pid
			children[pid] = append(children[pid], id)
		}
	}
	prows, err := db.Query(`SELECT user_id, COALESCE(SUM(amount),0) FROM locations GROUP BY user_id`)
	if err != nil {
		return err
	}
	defer prows.Close()
	for prows.Next() {
		var uid uint64
		var sum string
		if err := prows.Scan(&uid, &sum); err != nil {
			return err
		}
		personal[uid], _ = decimal.NewFromString(sum)
	}

	bad := 0
	for _, c := range cases {
		cid, ok := ids[c.claimant]
		if !ok {
			fmt.Printf("FAIL [peer] missing claimant %s\n", c.claimant)
			bad++
			continue
		}
		sid, ok := ids[c.peerSrc]
		if !ok {
			fmt.Printf("FAIL [peer] missing peerSrc %s\n", c.peerSrc)
			bad++
			continue
		}
		static := map[uint64]decimal.Decimal{sid: decimal.NewFromInt(1)}
		got := calcCommunityRewardsLocal(cid, children, parent, levels, personal, static)
		if !got.Peer.IsPositive() {
			fmt.Printf("FAIL [peer] %s should get peer from %s (same-level in small area)\n", c.claimant, c.peerSrc)
			bad++
		} else {
			fmt.Printf("OK  [peer] %s <- %s peer=%s\n", c.claimant, c.peerSrc, got.Peer.String())
		}
	}
	if bad > 0 {
		return fmt.Errorf("%d peer topology checks failed", bad)
	}
	return nil
}

type communitySplitLocal struct {
	Base decimal.Decimal
	Peer decimal.Decimal
}

func calcCommunityRewardsLocal(
	claimant uint64,
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	levels map[uint64]int,
	personal map[uint64]decimal.Decimal,
	todayStatic map[uint64]decimal.Decimal,
) communitySplitLocal {
	const minPeerLevel = 3
	peerRate := decimal.RequireFromString("0.1")
	selfLevel := levels[claimant]
	rateByLv := map[int]string{1: "0.10", 2: "0.20", 3: "0.30", 4: "0.35", 5: "0.40", 6: "0.45", 7: "0.50", 8: "0.55", 9: "0.60"}
	ru := decimal.Zero
	if s, ok := rateByLv[selfLevel]; ok {
		ru = decimal.RequireFromString(s)
	}
	out := communitySplitLocal{}
	if !ru.IsPositive() {
		return out
	}
	legs := children[claimant]
	if len(legs) == 0 {
		return out
	}
	legVols := make([]decimal.Decimal, len(legs))
	for i, leg := range legs {
		legVols[i] = subtreeVolume(leg, children, personal)
	}
	maxIdx := 0
	for i := 1; i < len(legVols); i++ {
		if legVols[i].GreaterThan(legVols[maxIdx]) {
			maxIdx = i
		}
	}
	claimantPeerOK := selfLevel >= minPeerLevel
	base := decimal.Zero
	peer := decimal.Zero
	for i, legRoot := range legs {
		if i == maxIdx {
			continue
		}
		ids := collectSubtreeIDsLocal(legRoot, children)
		for _, x := range ids {
			if underBreakLocal(x, claimant, selfLevel, parent, levels) {
				continue
			}
			s := todayStatic[x]
			if !s.IsPositive() {
				continue
			}
			lv := levels[x]
			if lv == selfLevel && claimantPeerOK && lv >= minPeerLevel {
				peer = peer.Add(s.Mul(peerRate))
				continue
			}
			gov := nearestGovLocal(x, claimant, selfLevel, parent, levels, rateByLv)
			diff := ru.Sub(gov)
			if diff.IsPositive() {
				base = base.Add(s.Mul(diff))
			}
		}
	}
	out.Base = base.Round(8)
	out.Peer = peer.Round(8)
	return out
}

func underBreakLocal(node, claimant uint64, claimantLevel int, parent map[uint64]uint64, levels map[uint64]int) bool {
	cur := node
	for cur != claimant {
		if levels[cur] > claimantLevel {
			return true
		}
		p, ok := parent[cur]
		if !ok {
			return false
		}
		cur = p
	}
	return false
}

func nearestGovLocal(node, claimant uint64, claimantLevel int, parent map[uint64]uint64, levels map[uint64]int, rateByLv map[int]string) decimal.Decimal {
	cur := node
	last := decimal.Zero
	for cur != claimant {
		if lv := levels[cur]; lv > 0 && lv != claimantLevel {
			if s, ok := rateByLv[lv]; ok {
				last = decimal.RequireFromString(s)
			}
		}
		p, ok := parent[cur]
		if !ok {
			return last
		}
		if p == claimant {
			return last
		}
		cur = p
	}
	return last
}

func collectSubtreeIDsLocal(root uint64, children map[uint64][]uint64) []uint64 {
	out := []uint64{root}
	stack := append([]uint64(nil), children[root]...)
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		out = append(out, n)
		stack = append(stack, children[n]...)
	}
	return out
}

func locBuy(amount string) locSeed {
	return locBuyDir(amount, "up")
}

func locBuyDir(amount, dir string) locSeed {
	amt := decimal.RequireFromString(amount)
	var mult decimal.Decimal
	switch {
	case amt.LessThan(decimal.NewFromInt(1000)):
		mult = decimal.RequireFromString("2")
	case amt.LessThan(decimal.NewFromInt(3000)):
		mult = decimal.RequireFromString("2.5")
	default:
		mult = decimal.RequireFromString("3")
	}
	exit := amt.Mul(mult)
	return locSeed{
		Amount: amount, Multiplier: mult.String(), ExitTarget: exit.StringFixed(8),
		Accumulated: "0", RatePercent: "0.60", Direction: dir, Status: "active",
	}
}

func locExited(amount, exit string) locSeed {
	return locSeed{
		Amount: amount, Multiplier: "2", ExitTarget: exit, Accumulated: exit,
		RatePercent: "1.20", Direction: "up", Status: "exited",
	}
}

func addr(n int) string {
	return fmt.Sprintf("0xd0000000000000000000000000000000000000%02x", n)
}

func cleanupOrphanDemos(db *sql.DB, demos []demoUser) error {
	keep := map[string]bool{}
	for _, d := range demos {
		keep[d.Address] = true
	}
	rows, err := db.Query(`SELECT id, address FROM users WHERE address LIKE '0xd000%'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type orphan struct {
		id   uint64
		addr string
	}
	var orphans []orphan
	for rows.Next() {
		var o orphan
		if err := rows.Scan(&o.id, &o.addr); err != nil {
			return err
		}
		if !keep[o.addr] {
			orphans = append(orphans, o)
		}
	}
	for _, o := range orphans {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		for _, q := range []string{
			`DELETE FROM ledger_entries WHERE user_id=?`,
			`DELETE FROM withdraws WHERE user_id=?`,
			`DELETE FROM locations WHERE user_id=?`,
			`DELETE FROM user_recommends WHERE user_id=?`,
		} {
			if _, err := tx.Exec(q, o.id); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
		if _, err := tx.Exec(`UPDATE users SET inviter_id=NULL WHERE inviter_id=?`, o.id); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.Exec(`DELETE FROM users WHERE id=?`, o.id); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		fmt.Printf("OK  [cleanup] removed orphan id=%d addr=%s\n", o.id, o.addr)
	}
	return nil
}

func seedOrder(demos []demoUser) []string {
	done := map[string]bool{}
	var out []string
	for len(out) < len(demos) {
		progress := false
		for _, d := range demos {
			if done[d.Label] {
				continue
			}
			if d.Parent == "" || done[d.Parent] {
				out = append(out, d.Label)
				done[d.Label] = true
				progress = true
			}
		}
		if !progress {
			panic("cycle in demo parent graph")
		}
	}
	return out
}

func clearRelated(tx *sql.Tx, userID uint64) error {
	if _, err := tx.Exec(`DELETE FROM ledger_entries WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM withdraws WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM locations WHERE user_id = ?`, userID); err != nil {
		return err
	}
	return nil
}

func upsertPath(tx *sql.Tx, userID uint64, path string) error {
	var id uint64
	err := tx.QueryRow(`SELECT id FROM user_recommends WHERE user_id = ?`, userID).Scan(&id)
	now := time.Now()
	if err == sql.ErrNoRows {
		_, err = tx.Exec(`INSERT INTO user_recommends (user_id, path, created_at, updated_at) VALUES (?, ?, ?, ?)`,
			userID, path, now, now)
		return err
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE user_recommends SET path = ?, updated_at = ? WHERE user_id = ?`, path, now, userID)
	return err
}

func getPath(db *sql.DB, userID uint64) (string, error) {
	var path string
	err := db.QueryRow(`SELECT path FROM user_recommends WHERE user_id = ?`, userID).Scan(&path)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return path, err
}

func getPathTx(tx *sql.Tx, userID uint64) (string, error) {
	var path string
	err := tx.QueryRow(`SELECT path FROM user_recommends WHERE user_id = ?`, userID).Scan(&path)
	if err == sql.ErrNoRows {
		return strconv.FormatUint(userID, 10), nil
	}
	return path, err
}

func loadDSN() (string, error) {
	candidates := []string{
		"app/app/configs/config.yaml",
		filepath.Join("..", "app", "app", "configs", "config.yaml"),
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg struct {
			Data struct {
				Database struct {
					Source string `yaml:"source"`
				} `yaml:"database"`
			} `yaml:"data"`
		}
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return "", err
		}
		if cfg.Data.Database.Source != "" {
			return cfg.Data.Database.Source, nil
		}
	}
	return "root:root@tcp(127.0.0.1:3306)/jinniu?charset=utf8mb4&parseTime=True&loc=Local", nil
}
