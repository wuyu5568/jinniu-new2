//go:build ignore

package main

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	db, err := sql.Open("mysql", "root:root@tcp(127.0.0.1:3306)/jinniu?charset=utf8mb4&parseTime=true")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	names := map[string]string{
		"extract_fee_rate":         "提取手续费比例",
		"generation_rate":          "代数奖比例",
		"max_generation_depth":     "最大代数深度",
		"peer_pool_rate":           "平级奖比例",
		"min_peer_level":           "平级最低等级(V)",
		"rate_min":                 "静态利率下限(%)",
		"rate_max":                 "静态利率上限(%)",
		"rate_step":                "静态利率步进",
		"min_subscribe_amount":     "最低认购金额",
		"multiplier_1_lt":          "出局倍数档1上限",
		"multiplier_1_multiplier":  "出局倍数档1倍数",
		"multiplier_2_lt":          "出局倍数档2上限",
		"multiplier_2_multiplier":  "出局倍数档2倍数",
		"multiplier_3_lt":          "出局倍数档3上限",
		"multiplier_3_multiplier":  "出局倍数档3倍数",
		"community_v9_min_volume":  "V9业绩门槛",
		"community_v9_rate":        "V9社区基础奖比例",
		"community_v8_min_volume":  "V8业绩门槛",
		"community_v8_rate":        "V8社区基础奖比例",
		"community_v7_min_volume":  "V7业绩门槛",
		"community_v7_rate":        "V7社区基础奖比例",
		"community_v6_min_volume":  "V6业绩门槛",
		"community_v6_rate":        "V6社区基础奖比例",
		"community_v5_min_volume":  "V5业绩门槛",
		"community_v5_rate":        "V5社区基础奖比例",
		"community_v4_min_volume":  "V4业绩门槛",
		"community_v4_rate":        "V4社区基础奖比例",
		"community_v3_min_volume":  "V3业绩门槛",
		"community_v3_rate":        "V3社区基础奖比例",
		"community_v2_min_volume":  "V2业绩门槛",
		"community_v2_rate":        "V2社区基础奖比例",
		"community_v1_min_volume":  "V1业绩门槛",
		"community_v1_rate":        "V1社区基础奖比例",
	}
	for k, n := range names {
		if _, err := db.Exec("UPDATE business_configs SET name=? WHERE config_key=?", n, k); err != nil {
			panic(err)
		}
	}
	var name string
	_ = db.QueryRow("SELECT name FROM business_configs WHERE config_key='extract_fee_rate'").Scan(&name)
	fmt.Println("sample:", name)
}
