/**
 * Copyright 2025-present Coinbase Global, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package fees

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// FeeStrategy calculates percentage-based trading fees
type FeeStrategy struct {
	Percent decimal.Decimal // e.g., 0.001 for 0.1% (10 bps)
}

// NewFeeStrategy creates a new percentage-based fee strategy
func NewFeeStrategy(percent decimal.Decimal) *FeeStrategy {
	return &FeeStrategy{Percent: percent}
}

// Compute calculates the fee for a given quantity and price
func (s *FeeStrategy) Compute(qty decimal.Decimal, price decimal.Decimal) decimal.Decimal {
	notional := qty.Mul(price)
	return s.ComputeFromNotional(notional)
}

// ComputeFromNotional calculates the fee from a notional value (qty * price)
// This is used for quote-denominated orders where we already know the total value
func (s *FeeStrategy) ComputeFromNotional(notional decimal.Decimal) decimal.Decimal {
	return notional.Mul(s.Percent)
}

// PriceAdjuster applies fee strategy to market prices
type PriceAdjuster struct {
	FeeStrategy *FeeStrategy
}

func NewPriceAdjuster(feeStrategy *FeeStrategy) *PriceAdjuster {
	return &PriceAdjuster{
		FeeStrategy: feeStrategy,
	}
}

// AdjustBidPrice reduces bid price to account for fee when user is selling
func (a *PriceAdjuster) AdjustBidPrice(price, qty decimal.Decimal) decimal.Decimal {
	fee := a.FeeStrategy.Compute(qty, price)
	notional := qty.Mul(price)
	adjustedNotional := notional.Sub(fee)

	if qty.IsZero() {
		return price
	}

	return adjustedNotional.Div(qty)
}

// AdjustAskPrice increases ask price to account for fee when user is buying
func (a *PriceAdjuster) AdjustAskPrice(price, qty decimal.Decimal) decimal.Decimal {
	fee := a.FeeStrategy.Compute(qty, price)
	notional := qty.Mul(price)
	adjustedNotional := notional.Add(fee)

	if qty.IsZero() {
		return price
	}

	return adjustedNotional.Div(qty)
}

// ComputeFee calculates the fee for a given quantity and price
func (a *PriceAdjuster) ComputeFee(qty, price decimal.Decimal) decimal.Decimal {
	return a.FeeStrategy.Compute(qty, price)
}

// CreateFeeStrategy creates a percentage-based fee strategy from configuration
func CreateFeeStrategy(feePercent string) (*FeeStrategy, error) {
	percent, err := decimal.NewFromString(feePercent)
	if err != nil {
		return nil, fmt.Errorf("invalid fee percent: %w", err)
	}
	if percent.IsNegative() {
		return nil, fmt.Errorf("fee percent cannot be negative")
	}
	return NewFeeStrategy(percent), nil
}
