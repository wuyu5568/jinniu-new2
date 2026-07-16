package main

import (
	"database/sql"
	"fmt"
	"os"
	"sort"

	_ "github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
)

var tiers = []struct {
	Level int
	Min   string
	Rate  string
}{
	{9, "20000000", "0.60"}, {8, "10000000", "0.55"}, {7, "5000000", "0.50"},
	{6, "1500000", "0.45"}, {5, "500000", "0.40"}, {4, "250000", "0.35"},
	{3, "80000", "0.30"}, {2, "20000", "0.20"}, {1, "5000", "0.10"},
}

func levelFromVol(v decimal.Decimal) int {
	best := 0
	for _, t := range tiers {
		min := decimal.RequireFromString(t.Min)
		if v.GreaterThanOrEqual(min) && t.Level > best {
			best = t.Level
		}
	}
	return best
}

func rateFor(level int) decimal.Decimal {
	for _, t := range tiers {
		if t.Level == level {
			return decimal.RequireFromString(t.Rate)
		}
	}
	return decimal.Zero
}

func subtree(root uint64, children map[uint64][]uint64, personal map[uint64]decimal.Decimal) decimal.Decimal {
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

func collect(root uint64, children map[uint64][]uint64) []uint64 {
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

func maxIdx(vols []decimal.Decimal) int {
	m := 0
	for i := 1; i < len(vols); i++ {
		if vols[i].GreaterThan(vols[m]) {
			m = i
		}
	}
	return m
}

func smallArea(vols []decimal.Decimal) decimal.Decimal {
	if len(vols) <= 1 {
		return decimal.Zero
	}
	mi := maxIdx(vols)
	s := decimal.Zero
	for i, v := range vols {
		if i != mi {
			s = s.Add(v)
		}
	}
	return s
}

func underBreak(node, claimant uint64, cl int, parent map[uint64]uint64, levels map[uint64]int) bool {
	cur := node
	for cur != claimant {
		if levels[cur] >= cl {
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

func deepestGov(node, claimant uint64, parent map[uint64]uint64, levels map[uint64]int) decimal.Decimal {
	cur := node
	for {
		if lv := levels[cur]; lv > 0 {
			return rateFor(lv)
		}
		p, ok := parent[cur]
		if !ok || p == claimant {
			return decimal.Zero
		}
		cur = p
	}
}

func short(a string) string {
	if len(a) < 12 {
		return a
	}
	return a[:8] + "…" + a[len(a)-4:]
}

func main() {
	db, err := sql.Open("mysql", "root:root@tcp(127.0.0.1:3306)/jinniu?parseTime=true&loc=Local")
	must(err)
	defer db.Close()
	const claimant uint64 = 2

	children := map[uint64][]uint64{}
	parent := map[uint64]uint64{}
	var ids []uint64
	rows, err := db.Query(`SELECT id, inviter_id FROM users`)
	must(err)
	for rows.Next() {
		var id uint64
		var inv sql.NullInt64
		must(rows.Scan(&id, &inv))
		ids = append(ids, id)
		if inv.Valid {
			pid := uint64(inv.Int64)
			parent[id] = pid
			children[pid] = append(children[pid], id)
		}
	}
	rows.Close()

	personal := map[uint64]decimal.Decimal{}
	prows, err := db.Query(`SELECT user_id, COALESCE(SUM(amount),0) FROM locations GROUP BY user_id`)
	must(err)
	for prows.Next() {
		var uid uint64
		var s string
		must(prows.Scan(&uid, &s))
		personal[uid], _ = decimal.NewFromString(s)
	}
	prows.Close()
	for _, id := range ids {
		if _, ok := personal[id]; !ok {
			personal[id] = decimal.Zero
		}
	}

	levels := map[uint64]int{}
	for _, id := range ids {
		legs := children[id]
		vols := make([]decimal.Decimal, len(legs))
		for i, leg := range legs {
			vols[i] = subtree(leg, children, personal)
		}
		levels[id] = levelFromVol(smallArea(vols))
	}

	todayStatic := map[uint64]decimal.Decimal{}
	srows, err := db.Query(`
		SELECT user_id, SUM(amount) FROM ledger_entries
		WHERE entry_type='static' AND created_at BETWEEN '2026-07-15 18:31:18' AND '2026-07-15 18:31:21'
		GROUP BY user_id`)
	must(err)
	for srows.Next() {
		var uid uint64
		var s string
		must(srows.Scan(&uid, &s))
		todayStatic[uid], _ = decimal.NewFromString(s)
	}
	srows.Close()

	addrOf := map[uint64]string{}
	arows, err := db.Query(`SELECT id, address FROM users`)
	must(err)
	for arows.Next() {
		var id uint64
		var a string
		must(arows.Scan(&id, &a))
		addrOf[id] = a
	}
	arows.Close()

	selfLevel := levels[claimant]
	ru := rateFor(selfLevel)
	fmt.Printf("领取人 uid=%d  V%d  比例 r_u=%s\n", claimant, selfLevel, ru)

	legs := children[claimant]
	vols := make([]decimal.Decimal, len(legs))
	for i, leg := range legs {
		vols[i] = subtree(leg, children, personal)
	}
	mi := maxIdx(vols)
	fmt.Printf("直推腿数=%d  大区腿=uid:%d %s 业绩=%s（整腿不计社区奖）\n", len(legs), legs[mi], short(addrOf[legs[mi]]), vols[mi])
	fmt.Printf("小区业绩=%s → V%d\n\n", smallArea(vols), selfLevel)

	type line struct {
		uid, leg uint64
		static, gov, diff, amt decimal.Decimal
		brk bool
	}
	var lines []line
	total := decimal.Zero
	skipMax := decimal.Zero

	for i, legRoot := range legs {
		if i == mi {
			for _, x := range collect(legRoot, children) {
				if s := todayStatic[x]; s.IsPositive() {
					skipMax = skipMax.Add(s)
				}
			}
			continue
		}
		for _, x := range collect(legRoot, children) {
			s := todayStatic[x]
			if !s.IsPositive() {
				continue
			}
			if underBreak(x, claimant, selfLevel, parent, levels) {
				lines = append(lines, line{uid: x, leg: legRoot, static: s, brk: true})
				continue
			}
			gov := deepestGov(x, claimant, parent, levels)
			diff := ru.Sub(gov)
			if !diff.IsPositive() {
				continue
			}
			amt := s.Mul(diff).Round(8)
			total = total.Add(amt)
			lines = append(lines, line{uid: x, leg: legRoot, static: s, gov: gov, diff: diff, amt: amt})
		}
	}
	total = total.Round(8)
	fmt.Printf("重算合计=%s  流水记账=68822.22500000  大区腿内当日静态合计(不计)=%s\n\n", total, skipMax)

	sort.Slice(lines, func(i, j int) bool {
		if lines[i].brk != lines[j].brk {
			return !lines[i].brk
		}
		return lines[i].amt.GreaterThan(lines[j].amt)
	})

	fmt.Println("【计奖明细】奖额 = 当日静态 × (0.55 − 治理比例g)")
	fmt.Println("uid\t自身V\t直推腿\t当日静态\tg\t级差\t奖额\t地址尾号")
	n := 0
	for _, L := range lines {
		if L.brk {
			continue
		}
		n++
		fmt.Printf("%d\t%d\t%d\t%s\t%s\t%s\t%s\t%s\n",
			L.uid, levels[L.uid], L.leg, L.static.StringFixed(4), L.gov.StringFixed(2), L.diff.StringFixed(2), L.amt.StringFixed(4), short(addrOf[L.uid]))
	}
	fmt.Printf("计奖行数=%d\n\n", n)

	fmt.Println("【断档跳过】路上有 V≥8，静态不发给领取人（前10条）")
	c := 0
	for _, L := range lines {
		if !L.brk {
			continue
		}
		fmt.Printf("uid=%d 自身V=%d 静态=%s %s\n", L.uid, levels[L.uid], L.static, short(addrOf[L.uid]))
		c++
		if c >= 10 {
			break
		}
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
