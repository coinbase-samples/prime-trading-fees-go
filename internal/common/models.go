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

package common

import (
	"time"

	"github.com/coinbase-samples/prime-sdk-go/orders"
	"github.com/shopspring/decimal"
)

// ============================================================================
// RFQ Models
// ============================================================================

// RfqRequest represents a request for quote from the user's perspective
type RfqRequest struct {
	Product    string          // e.g., "BTC-USD"
	Side       string          // "BUY" or "SELL"
	BaseQty    decimal.Decimal // For base-denominated RFQs
	QuoteValue decimal.Decimal // For quote-denominated RFQs
	LimitPrice decimal.Decimal // Optional limit price
	Unit       string          // "base" or "quote"
}

// RfqResponse represents the quote response shown to the user
type RfqResponse struct {
	QuoteId             string `json:"quote_id"`
	Product             string `json:"product"`
	Side                string `json:"side"`
	ExpirationTime      string `json:"expiration_time"`
	Unit                string `json:"unit"`
	UserRequestedAmount string `json:"user_requested_amount"`
	Timestamp           string `json:"timestamp"`
	RawPrimeQuote       struct {
		BestPrice            string `json:"best_price"`
		OrderTotal           string `json:"order_total"`
		PriceInclusiveOfFees string `json:"price_inclusive_of_fees"`
	} `json:"raw_prime_quote"`
	CustomFeeOverlay struct {
		FeeAmount      string `json:"fee_amount"`
		FeePercent     string `json:"fee_percent"`
		EffectivePrice string `json:"effective_price"`
		TotalCost      string `json:"total_cost"`
	} `json:"custom_fee_overlay"`
}

// AcceptRfqRequest represents the request to accept a quote
type AcceptRfqRequest struct {
	QuoteId       string
	Product       string
	Side          string
	ClientOrderId string
}

// AcceptRfqResponse represents the response after accepting a quote
type AcceptRfqResponse struct {
	OrderId       string `json:"order_id"`
	QuoteId       string `json:"quote_id"`
	ClientOrderId string `json:"client_order_id"`
	Product       string `json:"product"`
	Side          string `json:"side"`
}

// ============================================================================
// Order Models
// ============================================================================

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

// ============================================================================
// Market Data Models
// ============================================================================

// PriceLevel represents a single price level in the order book
type PriceLevel struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

// OrderBookSnapshot is an immutable snapshot of the order book
type OrderBookSnapshot struct {
	Product    string
	Bids       []PriceLevel
	Asks       []PriceLevel
	UpdateTime time.Time
	Sequence   uint64
}
