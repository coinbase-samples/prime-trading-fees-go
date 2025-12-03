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

package main

import (
	"fmt"
	"strings"

	"github.com/coinbase-samples/prime-trading-fees-go/internal/common"
	"github.com/shopspring/decimal"
)

// parsedFlags holds the validated and normalized command line flags
type parsedFlags struct {
	symbol     string
	side       string
	orderType  string
	unitType   string
	quantity   decimal.Decimal
	limitPrice decimal.Decimal
	isPreview  bool
}

// parseAndValidateFlags parses and validates all command line flags
func parseAndValidateFlags(symbolVal, sideVal, qtyVal, unitVal, orderTypeVal, priceVal, modeVal string) (*parsedFlags, error) {
	// Validate required flags
	if symbolVal == "" {
		return nil, fmt.Errorf("--symbol is required")
	}
	if sideVal == "" {
		return nil, fmt.Errorf("--side is required (buy or sell)")
	}
	if qtyVal == "" {
		return nil, fmt.Errorf("--qty is required")
	}

	// Normalize and validate side
	sideUpper := common.NormalizeSide(sideVal)
	if sideUpper != "BUY" && sideUpper != "SELL" {
		return nil, fmt.Errorf("--side must be 'buy' or 'sell', got: %s", sideVal)
	}

	// Determine unit with smart defaults
	unitType := unitVal
	if unitType == "" {
		// Smart defaults: buy in quote (USD), sell in base (BTC/ETH)
		if sideUpper == "BUY" {
			unitType = "quote"
		} else {
			unitType = "base"
		}
	}

	// Validate and normalize unit
	if strings.EqualFold(unitType, "base") {
		unitType = "base"
	} else if strings.EqualFold(unitType, "quote") {
		unitType = "quote"
	} else {
		return nil, fmt.Errorf("--unit must be 'base' or 'quote', got: %s", unitVal)
	}

	// Normalize and validate order type
	typeUpper := common.NormalizeOrderType(orderTypeVal)
	if typeUpper != "MARKET" && typeUpper != "LIMIT" {
		return nil, fmt.Errorf("--type must be 'market' or 'limit', got: %s", orderTypeVal)
	}

	// Validate and normalize mode
	isPreview := false
	modeValue := modeVal
	if strings.EqualFold(modeValue, "preview") {
		isPreview = true
	} else if strings.EqualFold(modeValue, "execute") {
		isPreview = false
	} else {
		return nil, fmt.Errorf("--mode must be 'preview' or 'execute', got: %s", modeVal)
	}

	// Parse quantity
	quantity, err := decimal.NewFromString(qtyVal)
	if err != nil {
		return nil, fmt.Errorf("invalid quantity: %w", err)
	}

	// Parse limit price if provided
	var limitPrice decimal.Decimal
	if priceVal != "" {
		limitPrice, err = decimal.NewFromString(priceVal)
		if err != nil {
			return nil, fmt.Errorf("invalid price: %w", err)
		}
	}

	// Validate type/price combination
	if typeUpper == "LIMIT" && priceVal == "" {
		return nil, fmt.Errorf("--price is required for limit orders")
	}
	if typeUpper == "MARKET" && priceVal != "" {
		return nil, fmt.Errorf("--price should not be specified for market orders")
	}

	return &parsedFlags{
		symbol:     symbolVal,
		side:       sideUpper,
		orderType:  typeUpper,
		unitType:   unitType,
		quantity:   quantity,
		limitPrice: limitPrice,
		isPreview:  isPreview,
	}, nil
}
