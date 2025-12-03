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
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestParseAndValidateOrderFlags(t *testing.T) {
	tests := []struct {
		name        string
		symbol      string
		side        string
		qty         string
		unit        string
		orderType   string
		price       string
		mode        string
		wantErr     bool
		errContains string
		validate    func(*testing.T, *parsedOrderFlags)
	}{
		// Happy path - market buy with quote (default)
		{
			name:      "market buy with default unit",
			symbol:    "BTC-USD",
			side:      "buy",
			qty:       "1000",
			unit:      "",
			orderType: "market",
			price:     "",
			mode:      "execute",
			wantErr:   false,
			validate: func(t *testing.T, flags *parsedOrderFlags) {
				if flags.symbol != "BTC-USD" {
					t.Errorf("expected symbol BTC-USD, got %s", flags.symbol)
				}
				if flags.side != "BUY" {
					t.Errorf("expected side BUY, got %s", flags.side)
				}
				if flags.unitType != "quote" {
					t.Errorf("expected unit quote, got %s", flags.unitType)
				}
				if !flags.quantity.Equal(decimal.NewFromInt(1000)) {
					t.Errorf("expected quantity 1000, got %s", flags.quantity)
				}
				if flags.isPreview {
					t.Error("expected isPreview false")
				}
			},
		},
		// Happy path - market sell with base (default)
		{
			name:      "market sell with default unit",
			symbol:    "ETH-USD",
			side:      "sell",
			qty:       "0.5",
			unit:      "",
			orderType: "market",
			price:     "",
			mode:      "preview",
			wantErr:   false,
			validate: func(t *testing.T, flags *parsedOrderFlags) {
				if flags.side != "SELL" {
					t.Errorf("expected side SELL, got %s", flags.side)
				}
				if flags.unitType != "base" {
					t.Errorf("expected unit base, got %s", flags.unitType)
				}
				if !flags.isPreview {
					t.Error("expected isPreview true")
				}
			},
		},
		// Happy path - limit buy with explicit price
		{
			name:      "limit buy with price",
			symbol:    "BTC-USD",
			side:      "BUY",
			qty:       "1000",
			unit:      "quote",
			orderType: "limit",
			price:     "50000",
			mode:      "execute",
			wantErr:   false,
			validate: func(t *testing.T, flags *parsedOrderFlags) {
				if flags.orderType != "LIMIT" {
					t.Errorf("expected type LIMIT, got %s", flags.orderType)
				}
				if !flags.limitPrice.Equal(decimal.NewFromInt(50000)) {
					t.Errorf("expected price 50000, got %s", flags.limitPrice)
				}
			},
		},
		// Case insensitivity for unit and mode
		{
			name:      "case insensitive unit and mode",
			symbol:    "BTC-USD",
			side:      "buy",
			qty:       "100",
			unit:      "QuOtE",
			orderType: "market",
			price:     "",
			mode:      "PrEvIeW",
			wantErr:   false,
			validate: func(t *testing.T, flags *parsedOrderFlags) {
				if flags.unitType != "quote" {
					t.Errorf("expected normalized unit quote, got %s", flags.unitType)
				}
				if !flags.isPreview {
					t.Error("expected isPreview true from case-insensitive 'PrEvIeW'")
				}
			},
		},
		// Explicit unit override on buy
		{
			name:      "buy with explicit base unit",
			symbol:    "BTC-USD",
			side:      "buy",
			qty:       "0.1",
			unit:      "base",
			orderType: "market",
			price:     "",
			mode:      "execute",
			wantErr:   false,
			validate: func(t *testing.T, flags *parsedOrderFlags) {
				if flags.unitType != "base" {
					t.Errorf("expected explicit unit base, got %s", flags.unitType)
				}
			},
		},
		// Missing required: symbol
		{
			name:        "missing symbol",
			symbol:      "",
			side:        "buy",
			qty:         "1000",
			unit:        "",
			orderType:   "market",
			price:       "",
			mode:        "execute",
			wantErr:     true,
			errContains: "--symbol is required",
		},
		// Missing required: side
		{
			name:        "missing side",
			symbol:      "BTC-USD",
			side:        "",
			qty:         "1000",
			unit:        "",
			orderType:   "market",
			price:       "",
			mode:        "execute",
			wantErr:     true,
			errContains: "--side is required",
		},
		// Missing required: qty
		{
			name:        "missing qty",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "",
			unit:        "",
			orderType:   "market",
			price:       "",
			mode:        "execute",
			wantErr:     true,
			errContains: "--qty is required",
		},
		// Invalid side
		{
			name:        "invalid side",
			symbol:      "BTC-USD",
			side:        "invalid",
			qty:         "1000",
			unit:        "",
			orderType:   "market",
			price:       "",
			mode:        "execute",
			wantErr:     true,
			errContains: "--side must be 'buy' or 'sell'",
		},
		// Invalid unit
		{
			name:        "invalid unit",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "1000",
			unit:        "invalid",
			orderType:   "market",
			price:       "",
			mode:        "execute",
			wantErr:     true,
			errContains: "--unit must be 'base' or 'quote'",
		},
		// Invalid order type
		{
			name:        "invalid order type",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "1000",
			unit:        "",
			orderType:   "invalid",
			price:       "",
			mode:        "execute",
			wantErr:     true,
			errContains: "--type must be 'market' or 'limit'",
		},
		// Invalid mode
		{
			name:        "invalid mode",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "1000",
			unit:        "",
			orderType:   "market",
			price:       "",
			mode:        "invalid",
			wantErr:     true,
			errContains: "--mode must be 'preview' or 'execute'",
		},
		// Invalid quantity format
		{
			name:        "invalid quantity format",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "not-a-number",
			unit:        "",
			orderType:   "market",
			price:       "",
			mode:        "execute",
			wantErr:     true,
			errContains: "invalid quantity",
		},
		// Invalid price format
		{
			name:        "invalid price format",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "1000",
			unit:        "",
			orderType:   "limit",
			price:       "not-a-number",
			mode:        "execute",
			wantErr:     true,
			errContains: "invalid price",
		},
		// Limit order without price
		{
			name:        "limit order missing price",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "1000",
			unit:        "",
			orderType:   "limit",
			price:       "",
			mode:        "execute",
			wantErr:     true,
			errContains: "--price is required for limit orders",
		},
		// Market order with price
		{
			name:        "market order with price",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "1000",
			unit:        "",
			orderType:   "market",
			price:       "50000",
			mode:        "execute",
			wantErr:     true,
			errContains: "--price should not be specified for market orders",
		},
		// Decimal quantity
		{
			name:      "decimal quantity",
			symbol:    "BTC-USD",
			side:      "buy",
			qty:       "0.00123456",
			unit:      "base",
			orderType: "market",
			price:     "",
			mode:      "execute",
			wantErr:   false,
			validate: func(t *testing.T, flags *parsedOrderFlags) {
				expected := decimal.RequireFromString("0.00123456")
				if !flags.quantity.Equal(expected) {
					t.Errorf("expected quantity %s, got %s", expected, flags.quantity)
				}
			},
		},
		// Large decimal price
		{
			name:      "large decimal price",
			symbol:    "BTC-USD",
			side:      "buy",
			qty:       "1000",
			unit:      "quote",
			orderType: "limit",
			price:     "99999.99",
			mode:      "execute",
			wantErr:   false,
			validate: func(t *testing.T, flags *parsedOrderFlags) {
				expected := decimal.RequireFromString("99999.99")
				if !flags.limitPrice.Equal(expected) {
					t.Errorf("expected price %s, got %s", expected, flags.limitPrice)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := parseAndValidateOrderFlags(
				tt.symbol,
				tt.side,
				tt.qty,
				tt.unit,
				tt.orderType,
				tt.price,
				tt.mode,
			)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errContains)
					return
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if flags == nil {
				t.Error("expected non-nil flags")
				return
			}

			if tt.validate != nil {
				tt.validate(t, flags)
			}
		})
	}
}
