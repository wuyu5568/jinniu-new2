package biz

import "github.com/shopspring/decimal"

const (
	MinPeerLevel = 3
	PeerPoolRate = "0.10"
)

// LevelFromVolume returns V1–V9 (1–9) or 0 when below V1.
func LevelFromVolume(smallAreaVolume decimal.Decimal) int {
	if !smallAreaVolume.IsPositive() {
		return 0
	}
	best := 0
	for _, t := range GetActiveParams().CommunityTiers {
		minVol := decimal.RequireFromString(t.MinVolume)
		if smallAreaVolume.GreaterThanOrEqual(minVol) && t.Level > best {
			best = t.Level
		}
	}
	return best
}

// RateForLevel returns the community base rate fraction for a V level (0 if none).
func RateForLevel(level int) decimal.Decimal {
	if level <= 0 {
		return decimal.Zero
	}
	for _, t := range GetActiveParams().CommunityTiers {
		if t.Level == level {
			return decimal.RequireFromString(t.Rate)
		}
	}
	return decimal.Zero
}

// PeerEligible reports whether a level may receive/pay 平级 (same-level) reward.
func PeerEligible(level int) bool {
	min := GetActiveParams().MinPeerLevel
	if min <= 0 {
		min = MinPeerLevel
	}
	return level >= min
}

// SmallAreaVolume is 小区业绩: sum of leg volumes excluding the largest leg.
func SmallAreaVolume(legVolumes []decimal.Decimal) decimal.Decimal {
	maxIdx := MaxLegIndex(legVolumes)
	if maxIdx < 0 || len(legVolumes) <= 1 {
		return decimal.Zero
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

// MaxLegIndex returns the index of the largest leg volume (first on ties).
func MaxLegIndex(legVolumes []decimal.Decimal) int {
	if len(legVolumes) == 0 {
		return -1
	}
	maxIdx := 0
	for i := 1; i < len(legVolumes); i++ {
		if legVolumes[i].GreaterThan(legVolumes[maxIdx]) {
			maxIdx = i
		}
	}
	return maxIdx
}

// LegBlocked reports whether any listed level ≥ selfLevel (legacy whole-leg check).
// Prefer underCommunityBreak for 社区基础奖结算.
func LegBlocked(selfLevel int, downlineLevels []int) bool {
	if selfLevel <= 0 {
		return false
	}
	for _, lv := range downlineLevels {
		if lv >= selfLevel {
			return true
		}
	}
	return false
}

// underCommunityBreak reports 断档：自奖源的上级起，路径上出现高于或等于领取人的 V 级。
// 只挡该同级/更高节点之下的下级；奖源本人不因自身等级断档（本人走正常级差或直推全额例外）。
func underCommunityBreak(node, claimant uint64, claimantLevel int, parent map[uint64]uint64, levels map[uint64]int) bool {
	if claimantLevel <= 0 {
		return false
	}
	p, ok := parent[node]
	if !ok {
		return false
	}
	cur := p
	for cur != claimant {
		if levels[cur] >= claimantLevel {
			return true
		}
		next, ok := parent[cur]
		if !ok {
			return false
		}
		cur = next
	}
	return false
}

// SubtreeVolume sums personal volumes of root and all descendants.
func SubtreeVolume(root uint64, children map[uint64][]uint64, personal map[uint64]decimal.Decimal) decimal.Decimal {
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

// CollectSubtreeIDs returns root plus all descendants.
func CollectSubtreeIDs(root uint64, children map[uint64][]uint64) []uint64 {
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

// CommunityRewardSplit is 级差 + 平级 for one claimant.
type CommunityRewardSplit struct {
	Base decimal.Decimal
	Peer decimal.Decimal
}

const (
	CommunityLineBase = "base"
	CommunityLinePeer = "peer"
)

// StaticPiece is one order's static yield within a settle run (for ledger source buy/rate).
type StaticPiece struct {
	UserID      uint64
	OrderID     uint64
	Yield       decimal.Decimal
	BuyAmount   decimal.Decimal
	RatePercent decimal.Decimal // e.g. 0.75
}

// CommunityRewardLine is one per-source community / peer credit (for ledger split).
type CommunityRewardLine struct {
	SourceUserID  uint64
	SourceOrderID uint64
	Kind          string // CommunityLineBase | CommunityLinePeer
	Amount        decimal.Decimal
	SourceStatic  decimal.Decimal // base: source static; peer: source's community-base total
	SourceBuy     decimal.Decimal
	SourceRatePct decimal.Decimal // e.g. 0.75
	GapRate       decimal.Decimal // base: differential fraction; peer: peer pool rate
}

// CalcCommunityBase computes 社区基础奖 (级差 only) for claimant.
func CalcCommunityBase(
	claimant uint64,
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	levels map[uint64]int,
	todayStatic map[uint64]decimal.Decimal,
) decimal.Decimal {
	return CalcCommunityRewards(claimant, children, parent, levels, todayStatic).Base
}

// CalcCommunityRewards computes 级差 for claimant only（平级需全局社区奖，见 ComputeAllCommunityRewards）.
func CalcCommunityRewards(
	claimant uint64,
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	levels map[uint64]int,
	todayStatic map[uint64]decimal.Decimal,
) CommunityRewardSplit {
	out := CommunityRewardSplit{}
	for _, line := range ListCommunityRewardLines(claimant, children, parent, levels, todayStatic) {
		if line.Kind == CommunityLineBase {
			out.Base = out.Base.Add(line.Amount)
		}
	}
	out.Base = out.Base.Round(8)
	return out
}

// ListCommunityRewardLines returns per-source 级差 lines (user-aggregated static). Peer is separate.
func ListCommunityRewardLines(
	claimant uint64,
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	levels map[uint64]int,
	todayStatic map[uint64]decimal.Decimal,
) []CommunityRewardLine {
	pieces := make(map[uint64][]StaticPiece, len(todayStatic))
	for uid, s := range todayStatic {
		if s.IsPositive() {
			pieces[uid] = []StaticPiece{{UserID: uid, Yield: s}}
		}
	}
	return ListCommunityBaseLinesFromPieces(claimant, children, parent, levels, pieces)
}

// ListCommunityRewardLinesFromPieces is kept for callers; returns 级差 only.
func ListCommunityRewardLinesFromPieces(
	claimant uint64,
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	levels map[uint64]int,
	pieces map[uint64][]StaticPiece,
) []CommunityRewardLine {
	return ListCommunityBaseLinesFromPieces(claimant, children, parent, levels, pieces)
}

// ListCommunityBaseLinesFromPieces returns 级差 lines per source order static piece.
// 对照 = 奖源与领取人中间（不含两端）的最高 V 比例；断档只挡同级/更高节点的下级。
// 例外：直推本人静态不走对照/断档，一律 × 领取人当前 V 比例。
func ListCommunityBaseLinesFromPieces(
	claimant uint64,
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	levels map[uint64]int,
	pieces map[uint64][]StaticPiece,
) []CommunityRewardLine {
	selfLevel := levels[claimant]
	ru := RateForLevel(selfLevel)
	if !ru.IsPositive() {
		return nil
	}

	legs := children[claimant]
	if len(legs) == 0 {
		return nil
	}

	var lines []CommunityRewardLine
	for _, legRoot := range legs {
		ids := CollectSubtreeIDs(legRoot, children)
		for _, x := range ids {
			directSelf := x == legRoot
			if !directSelf && underCommunityBreak(x, claimant, selfLevel, parent, levels) {
				continue
			}
			for _, piece := range pieces[x] {
				s := piece.Yield
				if !s.IsPositive() {
					continue
				}
				var diff decimal.Decimal
				if directSelf {
					diff = ru
				} else {
					govRate := maxGradedRateBetween(x, claimant, parent, levels)
					diff = ru.Sub(govRate)
				}
				if !diff.IsPositive() {
					continue
				}
				amt := s.Mul(diff).Round(8)
				if !amt.IsPositive() {
					continue
				}
				lines = append(lines, CommunityRewardLine{
					SourceUserID:  x,
					SourceOrderID: piece.OrderID,
					Kind:          CommunityLineBase,
					Amount:        amt,
					SourceStatic:  s,
					SourceBuy:     piece.BuyAmount,
					SourceRatePct: piece.RatePercent,
					GapRate:       diff,
				})
			}
		}
	}
	return lines
}

// maxGradedRateBetween returns the community rate of the highest V level strictly between
// node and claimant (excluding both ends). Zero if no intermediate graded node.
// Example: A(V8)-B(V3)-C(V5)-D(V9) → for A on D, gov is C's 40% (max of B,C), gap 15%.
func maxGradedRateBetween(node, claimant uint64, parent map[uint64]uint64, levels map[uint64]int) decimal.Decimal {
	p, ok := parent[node]
	if !ok {
		return decimal.Zero
	}
	bestLv := 0
	cur := p
	for cur != claimant {
		if lv := levels[cur]; lv > bestLv {
			bestLv = lv
		}
		next, ok := parent[cur]
		if !ok {
			break
		}
		cur = next
	}
	if bestLv <= 0 {
		return decimal.Zero
	}
	return RateForLevel(bestLv)
}

// NearestSameLevelUpline returns the closest ancestor with the same community level, or 0.
func NearestSameLevelUpline(node uint64, parent map[uint64]uint64, levels map[uint64]int) uint64 {
	lv := levels[node]
	if lv <= 0 {
		return 0
	}
	cur := node
	for {
		p, ok := parent[cur]
		if !ok {
			return 0
		}
		if levels[p] == lv {
			return p
		}
		cur = p
	}
}

// PeerRewardLine is 平级：recipient gets fromUser's community-base total × peer rate.
type PeerRewardLine struct {
	RecipientID   uint64
	FromUserID    uint64
	Amount        decimal.Decimal
	CommunityBase decimal.Decimal
	PeerRate      decimal.Decimal
}

// ListPeerLinesFromCommunityBase builds 平级 after 社区基础奖 totals are known.
// 仅直推上级：上级等级 ≤ 来源等级，双方 ≥ min_peer_level；基数只含社区基础奖。
func ListPeerLinesFromCommunityBase(
	communityBaseByUser map[uint64]decimal.Decimal,
	parent map[uint64]uint64,
	levels map[uint64]int,
) []PeerRewardLine {
	peerRate := GetActiveParams().peerPoolRateDec()
	if !peerRate.IsPositive() {
		return nil
	}
	var lines []PeerRewardLine
	for fromID, base := range communityBaseByUser {
		if !base.IsPositive() {
			continue
		}
		fromLv := levels[fromID]
		if !PeerEligible(fromLv) {
			continue
		}
		recipient, ok := parent[fromID]
		if !ok || recipient == 0 {
			continue
		}
		recLv := levels[recipient]
		if recLv > fromLv || !PeerEligible(recLv) {
			continue
		}
		amt := base.Mul(peerRate).Round(8)
		if !amt.IsPositive() {
			continue
		}
		lines = append(lines, PeerRewardLine{
			RecipientID:   recipient,
			FromUserID:    fromID,
			Amount:        amt,
			CommunityBase: base,
			PeerRate:      peerRate,
		})
	}
	return lines
}

// ComputeAllCommunityRewards computes 级差 per user then 平级 from community-base totals.
func ComputeAllCommunityRewards(
	userIDs []uint64,
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	levels map[uint64]int,
	todayStatic map[uint64]decimal.Decimal,
) map[uint64]CommunityRewardSplit {
	pieces := make(map[uint64][]StaticPiece, len(todayStatic))
	for uid, s := range todayStatic {
		if s.IsPositive() {
			pieces[uid] = []StaticPiece{{UserID: uid, Yield: s}}
		}
	}
	out := make(map[uint64]CommunityRewardSplit, len(userIDs))
	baseByUser := make(map[uint64]decimal.Decimal, len(userIDs))
	for _, uid := range userIDs {
		for _, line := range ListCommunityBaseLinesFromPieces(uid, children, parent, levels, pieces) {
			s := out[uid]
			s.Base = s.Base.Add(line.Amount)
			out[uid] = s
			baseByUser[uid] = baseByUser[uid].Add(line.Amount)
		}
	}
	for _, pl := range ListPeerLinesFromCommunityBase(baseByUser, parent, levels) {
		s := out[pl.RecipientID]
		s.Peer = s.Peer.Add(pl.Amount)
		out[pl.RecipientID] = s
	}
	for uid, s := range out {
		s.Base = s.Base.Round(8)
		s.Peer = s.Peer.Round(8)
		out[uid] = s
	}
	return out
}
