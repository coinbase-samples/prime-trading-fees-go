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

	"github.com/coinbase-samples/prime-trading-fees-go/config"
	"github.com/shopspring/decimal"
)

// FeeStrategy defines the interface for computing trading fees
type FeeStrategy interface {
	// Compute calculates the fee for a given quantity and price
	Compute(qty decimal.Decimal, price decimal.Decimal) (fee decimal.Decimal)
	// ComputeFromNotional calculates the fee from a notional value (qty * price)
	// This is used for quote-denominated orders where we already know the total value
	ComputeFromNotional(notional decimal.Decimal) (fee decimal.Decimal)
	// Name returns the name of the strategy
	Name() string
}

// FlatFeeStrategy applies a fixed fee amount per trade
type FlatFeeStrategy struct {
	Amount decimal.Decimal
}

func NewFlatFeeStrategy(amount decimal.Decimal) *FlatFeeStrategy {
	return &FlatFeeStrategy{Amount: amount}
}

func (s *FlatFeeStrategy) Compute(qty decimal.Decimal, price decimal.Decimal) decimal.Decimal {
	return s.Amount
}

func (s *FlatFeeStrategy) ComputeFromNotional(notional decimal.Decimal) decimal.Decimal {
	return s.Amount
}

func (s *FlatFeeStrategy) Name() string {
	return "FlatFee"
}

// PercentFeeStrategy applies a percentage-based fee
type PercentFeeStrategy struct {
	Percent decimal.Decimal // e.g., 0.001 for 0.1%
}

func NewPercentFeeStrategy(percent decimal.Decimal) *PercentFeeStrategy {
	return &PercentFeeStrategy{Percent: percent}
}

func (s *PercentFeeStrategy) Compute(qty decimal.Decimal, price decimal.Decimal) decimal.Decimal {
	notional := qty.Mul(price)
	return s.ComputeFromNotional(notional)
}

func (s *PercentFeeStrategy) ComputeFromNotional(notional decimal.Decimal) decimal.Decimal {
	return notional.Mul(s.Percent)
}

func (s *PercentFeeStrategy) Name() string {
	return "PercentFee"
}

// PriceAdjuster applies fee strategy to market prices
type PriceAdjuster struct {
	FeeStrategy FeeStrategy
}

func NewPriceAdjuster(feeStrategy FeeStrategy) *PriceAdjuster {
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

// CreateFeeStrategy creates a fee strategy from configuration
func CreateFeeStrategy(cfg config.FeesConfig) (FeeStrategy, error) {
	switch cfg.Type {
	case "flat":
		amount, err := decimal.NewFromString(cfg.Amount)
		if err != nil {
			return nil, fmt.Errorf("invalid flat fee amount: %w", err)
		}
		return NewFlatFeeStrategy(amount), nil

	case "percent":
		percent, err := decimal.NewFromString(cfg.Percent)
		if err != nil {
			return nil, fmt.Errorf("invalid percent: %w", err)
		}
		return NewPercentFeeStrategy(percent), nil

	default:
		return nil, fmt.Errorf("unknown fee strategy type: %s", cfg.Type)
	}
}
