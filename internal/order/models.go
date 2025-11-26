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

package order

import (
	"time"

	"github.com/coinbase-samples/prime-sdk-go/orders"
	"github.com/shopspring/decimal"
)

// OrderRequest represents a request for an order (preview or actual)
type OrderRequest struct {
	Product    string
	Side       string          // "BUY" or "SELL"
	Type       string          // "MARKET", "LIMIT", etc.
	BaseQty    decimal.Decimal // Quantity in base currency (e.g., BTC, ETH)
	QuoteValue decimal.Decimal // Value in quote currency (e.g., USD)
	Price      decimal.Decimal // Optional, for limit orders
	Unit       string          // "base" or "quote" - indicates which field is populated
}

// OrderPreviewResponse contains the complete preview with fees
type OrderPreviewResponse struct {
	// Inputs
	Product             string `json:"product"`
	Side                string `json:"side"`
	Type                string `json:"type"`
	OrderUnit           string `json:"order_unit,omitempty"`            // How order was specified: "base" or "quote"
	UserRequestedAmount string `json:"user_requested_amount,omitempty"` // What user asked for (quote orders)
	RequestedPrice      string `json:"requested_price,omitempty"`       // For limit orders

	// Prime's response
	RawPreview *RawPrimePreview `json:"raw_prime_preview"`

	// Our overlay on top
	CustomFeeOverlay *CustomFeeOverlay `json:"custom_fee_overlay,omitempty"`

	Timestamp time.Time `json:"timestamp"`
}

// RawPrimePreview contains the raw response from Prime API
type RawPrimePreview struct {
	Quantity           string `json:"quantity"`             // BTC you're getting
	AverageFilledPrice string `json:"average_filled_price"` // Prime's execution price
	TotalValue         string `json:"total_value"`          // What we sent to Prime
	Commission         string `json:"commission"`           // Prime's fee
}

// CustomFeeOverlay contains our custom fee calculations on top of Prime's execution
type CustomFeeOverlay struct {
	FeeAmount      string `json:"fee_amount"`      // Our markup fee
	FeePercent     string `json:"fee_percent"`     // Our markup as percentage
	EffectivePrice string `json:"effective_price"` // True cost per BTC including all fees
}

// OrderResponse contains the response from placing an order
type OrderResponse struct {
	OrderId       string    `json:"order_id"`
	ClientOrderId string    `json:"client_order_id"`
	Product       string    `json:"product"`
	Side          string    `json:"side"`
	Type          string    `json:"type"`
	Status        string    `json:"status"`
	Timestamp     time.Time `json:"timestamp"`
}

// OrderMetadata contains calculated fee information for quote-denominated orders
type OrderMetadata struct {
	UserRequestedAmount   decimal.Decimal
	MarkupAmount          decimal.Decimal
	PrimeOrderQuoteAmount decimal.Decimal
}

// PreparedOrder contains the Prime API request and associated metadata
type PreparedOrder struct {
	PrimeRequest  *orders.CreateOrderRequest
	Metadata      *OrderMetadata
	NormalizedReq NormalizedOrderRequest
}

// NormalizedOrderRequest contains the normalized request parameters
type NormalizedOrderRequest struct {
	Product       string
	Side          string // Already normalized to uppercase
	Type          string // Already normalized to uppercase
	ClientOrderId string // Generated UUID
}
