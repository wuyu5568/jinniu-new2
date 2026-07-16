package biz

import (
	"context"
	"strings"
	"sync/atomic"

	"github.com/shopspring/decimal"
)

// CommunityTierParam is one V-level threshold in business params.
type CommunityTierParam struct {
	Level     int    `json:"level"`
	MinVolume string `json:"min_volume"`
	Rate      string `json:"rate"`
}

// MultiplierTierParam is one subscribe-amount → exit-multiplier band.
type MultiplierTierParam struct {
	Lt         string `json:"lt"`
	Multiplier string `json:"multiplier"`
}

// BusinessParams is the tunable business config persisted in MySQL.
type BusinessParams struct {
	ExtractFeeRate     string                `json:"extract_fee_rate"`
	GenerationRate     string                `json:"generation_rate"`
	MaxGenerationDepth int                   `json:"max_generation_depth"`
	PeerPoolRate       string                `json:"peer_pool_rate"`
	MinPeerLevel       int                   `json:"min_peer_level"`
	RateMin            string                `json:"rate_min"`
	RateMax            string                `json:"rate_max"`
	RateStep           string                `json:"rate_step"`
	MinSubscribeAmount string                `json:"min_subscribe_amount"`
	MinWithdrawAmount  string                `json:"min_withdraw_amount"`
	SubscribeTiers     string                `json:"subscribe_tiers"` // comma-separated preset amounts, e.g. 100,500,1000,3000
	MultiplierTiers    []MultiplierTierParam `json:"multiplier_tiers"`
	CommunityTiers     []CommunityTierParam  `json:"community_tiers"`
}

// BusinessConfig is one editable key/value business parameter row.
type BusinessConfig struct {
	ID    uint64
	Key   string
	Name  string
	Value string
}

// ParamsRepo loads / saves business params (aggregated from business_configs).
type ParamsRepo interface {
	Get(ctx context.Context) (*BusinessParams, error)
	Save(ctx context.Context, p *BusinessParams) error
	ListConfigs(ctx context.Context) ([]*BusinessConfig, error)
	UpdateConfigValue(ctx context.Context, id uint64, value string) (*BusinessConfig, error)
}

var activeParams atomic.Pointer[BusinessParams]

// DefaultBusinessParams returns the protocol defaults (金牛协议约定值).
func DefaultBusinessParams() BusinessParams {
	return BusinessParams{
		ExtractFeeRate:     "0.06",
		GenerationRate:     "0.05",
		MaxGenerationDepth: 10,
		PeerPoolRate:       "0.10",
		MinPeerLevel:       3,
		RateMin:            "0.60",
		RateMax:            "1.40",
		RateStep:           "0.05",
		MinSubscribeAmount: "100",
		MinWithdrawAmount:  "10",
		SubscribeTiers:     "100,500,1000,3000",
		MultiplierTiers: []MultiplierTierParam{
			{Lt: "1000", Multiplier: "2"},
			{Lt: "3000", Multiplier: "2.5"},
			{Lt: "", Multiplier: "3"},
		},
		CommunityTiers: []CommunityTierParam{
			{Level: 9, MinVolume: "20000000", Rate: "0.60"},
			{Level: 8, MinVolume: "10000000", Rate: "0.55"},
			{Level: 7, MinVolume: "5000000", Rate: "0.50"},
			{Level: 6, MinVolume: "1500000", Rate: "0.45"},
			{Level: 5, MinVolume: "500000", Rate: "0.40"},
			{Level: 4, MinVolume: "250000", Rate: "0.35"},
			{Level: 3, MinVolume: "80000", Rate: "0.30"},
			{Level: 2, MinVolume: "20000", Rate: "0.20"},
			{Level: 1, MinVolume: "5000", Rate: "0.10"},
		},
	}
}

// SetActiveParams replaces the in-process params used by settle / subscribe.
func SetActiveParams(p *BusinessParams) {
	if p == nil {
		d := DefaultBusinessParams()
		activeParams.Store(&d)
		return
	}
	cp := *p
	activeParams.Store(&cp)
}

// GetActiveParams returns the current in-process params (never nil).
func GetActiveParams() *BusinessParams {
	if p := activeParams.Load(); p != nil {
		return p
	}
	d := DefaultBusinessParams()
	activeParams.Store(&d)
	return &d
}

// ValidateBusinessParams checks required numeric fields parse and ranges look sane.
func ValidateBusinessParams(p *BusinessParams) error {
	if p == nil {
		return ErrInvalidAmount
	}
	for _, s := range []string{p.ExtractFeeRate, p.GenerationRate, p.PeerPoolRate, p.RateMin, p.RateMax, p.RateStep, p.MinSubscribeAmount, p.MinWithdrawAmount} {
		if _, err := decimal.NewFromString(s); err != nil {
			return ErrInvalidAmount
		}
	}
	if _, err := ParseSubscribeTiers(p.SubscribeTiers); err != nil {
		return ErrInvalidAmount
	}
	if p.MaxGenerationDepth < 1 || p.MaxGenerationDepth > 100 {
		return ErrInvalidAmount
	}
	if p.MinPeerLevel < 0 || p.MinPeerLevel > 9 {
		return ErrInvalidAmount
	}
	if len(p.MultiplierTiers) == 0 || len(p.CommunityTiers) == 0 {
		return ErrInvalidAmount
	}
	for _, t := range p.MultiplierTiers {
		if _, err := decimal.NewFromString(t.Multiplier); err != nil {
			return ErrInvalidAmount
		}
		if t.Lt != "" {
			if _, err := decimal.NewFromString(t.Lt); err != nil {
				return ErrInvalidAmount
			}
		}
	}
	for _, t := range p.CommunityTiers {
		if t.Level < 1 || t.Level > 9 {
			return ErrInvalidAmount
		}
		if _, err := decimal.NewFromString(t.MinVolume); err != nil {
			return ErrInvalidAmount
		}
		if _, err := decimal.NewFromString(t.Rate); err != nil {
			return ErrInvalidAmount
		}
	}
	return nil
}

func (p *BusinessParams) generationRateDec() decimal.Decimal {
	return decimal.RequireFromString(p.GenerationRate)
}

func (p *BusinessParams) peerPoolRateDec() decimal.Decimal {
	return decimal.RequireFromString(p.PeerPoolRate)
}

func (p *BusinessParams) rateMinDec() decimal.Decimal {
	return decimal.RequireFromString(p.RateMin)
}

func (p *BusinessParams) rateMaxDec() decimal.Decimal {
	return decimal.RequireFromString(p.RateMax)
}

func (p *BusinessParams) rateStepDec() decimal.Decimal {
	return decimal.RequireFromString(p.RateStep)
}

// ParseSubscribeTiers parses comma-separated amounts into sorted unique decimals.
// Empty string yields an empty list (custom input only).
func ParseSubscribeTiers(raw string) ([]decimal.Decimal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	seen := map[string]struct{}{}
	out := make([]decimal.Decimal, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		d, err := decimal.NewFromString(p)
		if err != nil || !d.IsPositive() {
			return nil, ErrInvalidAmount
		}
		key := d.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, d)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].LessThan(out[i]) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}
