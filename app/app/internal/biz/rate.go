package biz

import (
	"github.com/shopspring/decimal"
)

const (
	RateDirectionUp   = "up"
	RateDirectionDown = "down"

	OrderStatusActive = "active"
	OrderStatusExited = "exited"
)

// MultiplierForAmount returns the exit multiplier for a subscription amount.
func MultiplierForAmount(amount decimal.Decimal) (decimal.Decimal, bool) {
	p := GetActiveParams()
	min := decimal.RequireFromString(p.MinSubscribeAmount)
	if amount.LessThan(min) {
		return decimal.Zero, false
	}
	for _, t := range p.MultiplierTiers {
		if t.Lt == "" {
			return decimal.RequireFromString(t.Multiplier), true
		}
		lt := decimal.RequireFromString(t.Lt)
		if amount.LessThan(lt) {
			return decimal.RequireFromString(t.Multiplier), true
		}
	}
	return decimal.Zero, false
}

// InitialRate returns the first-day static rate for a new order.
func InitialRate() (rate decimal.Decimal, direction string) {
	return GetActiveParams().rateMinDec(), RateDirectionUp
}

// AdvanceRate moves one day along the configured rate cycle.
func AdvanceRate(current decimal.Decimal, direction string) (decimal.Decimal, string) {
	p := GetActiveParams()
	rateMin := p.rateMinDec()
	rateMax := p.rateMaxDec()
	rateStep := p.rateStepDec()

	if direction != RateDirectionDown {
		direction = RateDirectionUp
	}
	if direction == RateDirectionUp {
		next := current.Add(rateStep)
		if next.GreaterThan(rateMax) {
			next = rateMax.Sub(rateStep)
			if next.LessThan(rateMin) {
				next = rateMin
			}
			return next, RateDirectionDown
		}
		if next.Equal(rateMax) {
			return rateMax, RateDirectionDown
		}
		return next, RateDirectionUp
	}
	next := current.Sub(rateStep)
	if next.LessThan(rateMin) {
		next = rateMin.Add(rateStep)
		if next.GreaterThan(rateMax) {
			next = rateMax
		}
		return next, RateDirectionUp
	}
	if next.Equal(rateMin) {
		return rateMin, RateDirectionUp
	}
	return next, RateDirectionDown
}

// ReverseRateDirection flips climb/descend after extract.
func ReverseRateDirection(direction string) string {
	if direction == RateDirectionDown {
		return RateDirectionUp
	}
	return RateDirectionDown
}

// ApplyExtractRateTurn prepares next-day rate after extract approval.
func ApplyExtractRateTurn(current decimal.Decimal, direction string) (decimal.Decimal, string) {
	reversed := ReverseRateDirection(direction)
	return AdvanceRate(current, reversed)
}

// CalcStaticYield computes static yield: exitTarget * ratePercent / 100.
func CalcStaticYield(exitTarget, ratePercent decimal.Decimal) decimal.Decimal {
	return exitTarget.Mul(ratePercent).Div(decimal.NewFromInt(100)).Round(8)
}
