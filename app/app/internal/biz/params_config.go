package biz

import (
	"fmt"
	"strconv"
	"strings"
)

type configSeed struct {
	Key   string
	Name  string
	Value string
	Sort  int
}

// FlattenBusinessParams turns structured params into ordered config rows.
func FlattenBusinessParams(p *BusinessParams) []configSeed {
	if p == nil {
		d := DefaultBusinessParams()
		p = &d
	}
	rows := []configSeed{
		{"extract_fee_rate", "提现手续费比例", p.ExtractFeeRate, 10},
		{"generation_rate", "代数奖比例", p.GenerationRate, 20},
		{"max_generation_depth", "最大代数深度", strconv.Itoa(p.MaxGenerationDepth), 30},
		{"peer_pool_rate", "平级奖比例", p.PeerPoolRate, 40},
		{"min_peer_level", "平级最低等级(V)", strconv.Itoa(p.MinPeerLevel), 50},
		{"rate_min", "静态利率下限(%)", p.RateMin, 60},
		{"rate_max", "静态利率上限(%)", p.RateMax, 70},
		{"rate_step", "静态利率步进", p.RateStep, 80},
		{"min_subscribe_amount", "最低认购金额", p.MinSubscribeAmount, 90},
		{"min_withdraw_amount", "最低提现金额", p.MinWithdrawAmount, 91},
		{"subscribe_tiers", "认购档位(逗号分隔)", p.SubscribeTiers, 92},
	}
	for i, t := range p.MultiplierTiers {
		n := i + 1
		rows = append(rows,
			configSeed{fmt.Sprintf("multiplier_%d_lt", n), fmt.Sprintf("出局倍数档%d上限(空=末档)", n), t.Lt, 100 + i*2},
			configSeed{fmt.Sprintf("multiplier_%d_multiplier", n), fmt.Sprintf("出局倍数档%d倍数", n), t.Multiplier, 101 + i*2},
		)
	}
	for level := 9; level >= 1; level-- {
		t := findCommunityTier(p.CommunityTiers, level)
		if t == nil {
			continue
		}
		rows = append(rows,
			configSeed{fmt.Sprintf("community_v%d_min_volume", level), fmt.Sprintf("V%d业绩门槛", level), t.MinVolume, 200 + (9-level)*2},
			configSeed{fmt.Sprintf("community_v%d_rate", level), fmt.Sprintf("V%d社区基础奖比例", level), t.Rate, 201 + (9-level)*2},
		)
	}
	return rows
}

// ConfigLabels returns config_key → display name from current/default params shape.
func ConfigLabels() map[string]string {
	m := make(map[string]string)
	for _, s := range FlattenBusinessParams(GetActiveParams()) {
		m[s.Key] = s.Name
	}
	return m
}

func findCommunityTier(tiers []CommunityTierParam, level int) *CommunityTierParam {
	for i := range tiers {
		if tiers[i].Level == level {
			return &tiers[i]
		}
	}
	return nil
}

// AggregateBusinessParams rebuilds BusinessParams from config_key → value.
func AggregateBusinessParams(m map[string]string) (*BusinessParams, error) {
	def := DefaultBusinessParams()
	p := &BusinessParams{
		ExtractFeeRate:     pick(m, "extract_fee_rate", def.ExtractFeeRate),
		GenerationRate:     pick(m, "generation_rate", def.GenerationRate),
		PeerPoolRate:       pick(m, "peer_pool_rate", def.PeerPoolRate),
		RateMin:            pick(m, "rate_min", pick(m, "rate_mi", def.RateMin)),
		RateMax:            pick(m, "rate_max", def.RateMax),
		RateStep:           pick(m, "rate_step", def.RateStep),
		MinSubscribeAmount: pick(m, "min_subscribe_amount", def.MinSubscribeAmount),
		MinWithdrawAmount:  pick(m, "min_withdraw_amount", def.MinWithdrawAmount),
		SubscribeTiers:     pick(m, "subscribe_tiers", def.SubscribeTiers),
	}
	depth, err := strconv.Atoi(pick(m, "max_generation_depth", strconv.Itoa(def.MaxGenerationDepth)))
	if err != nil {
		return nil, ErrInvalidAmount
	}
	p.MaxGenerationDepth = depth
	minPeer, err := strconv.Atoi(pick(m, "min_peer_level", strconv.Itoa(def.MinPeerLevel)))
	if err != nil {
		return nil, ErrInvalidAmount
	}
	p.MinPeerLevel = minPeer

	var multipliers []MultiplierTierParam
	for i := 1; i <= 20; i++ {
		ltKey := fmt.Sprintf("multiplier_%d_lt", i)
		multKey := fmt.Sprintf("multiplier_%d_multiplier", i)
		lt, hasLt := m[ltKey]
		mult, hasMult := m[multKey]
		if !hasLt && !hasMult {
			break
		}
		if !hasMult || strings.TrimSpace(mult) == "" {
			return nil, ErrInvalidAmount
		}
		if !hasLt {
			lt = ""
		}
		multipliers = append(multipliers, MultiplierTierParam{Lt: lt, Multiplier: mult})
	}
	if len(multipliers) == 0 {
		multipliers = def.MultiplierTiers
	}
	p.MultiplierTiers = multipliers

	var communities []CommunityTierParam
	for level := 9; level >= 1; level-- {
		volKey := fmt.Sprintf("community_v%d_min_volume", level)
		rateKey := fmt.Sprintf("community_v%d_rate", level)
		vol, hasVol := m[volKey]
		rate, hasRate := m[rateKey]
		if !hasVol && !hasRate {
			continue
		}
		if !hasVol || !hasRate {
			return nil, ErrInvalidAmount
		}
		communities = append(communities, CommunityTierParam{Level: level, MinVolume: vol, Rate: rate})
	}
	if len(communities) == 0 {
		communities = def.CommunityTiers
	}
	p.CommunityTiers = communities
	return p, nil
}

func pick(m map[string]string, key, fallback string) string {
	if v, ok := m[key]; ok {
		return v
	}
	return fallback
}
