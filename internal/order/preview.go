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
	"context"
	"fmt"
	"time"

	"github.com/coinbase-samples/prime-sdk-go/client"
	"github.com/coinbase-samples/prime-sdk-go/credentials"
	"github.com/coinbase-samples/prime-sdk-go/orders"
	"github.com/coinbase-samples/prime-trading-fees-go/config"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/fees"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// OrderService handles order preview and placement logic
type OrderService struct {
	ordersSvc     orders.OrdersService
	portfolioId   string
	priceAdjuster *fees.PriceAdjuster
	metadataStore interface {
		Set(orderId string, metadata interface{})
	}
}

// NewOrderServiceWithPrime creates a new order service using Prime REST API
func NewOrderServiceWithPrime(cfg *config.Config, priceAdjuster *fees.PriceAdjuster, metadataStore interface{ Set(string, interface{}) }) *OrderService {
	creds := &credentials.Credentials{
		AccessKey:    cfg.Prime.AccessKey,
		Passphrase:   cfg.Prime.Passphrase,
		SigningKey:   cfg.Prime.SigningKey,
		PortfolioId:  cfg.Prime.Portfolio,
		SvcAccountId: cfg.Prime.ServiceAccountId,
	}

	httpClient, _ := client.DefaultHttpClient()
	restClient := client.NewRestClient(creds, httpClient)
	ordersSvc := orders.NewOrdersService(restClient)

	return &OrderService{
		ordersSvc:     ordersSvc,
		portfolioId:   cfg.Prime.Portfolio,
		priceAdjuster: priceAdjuster,
		metadataStore: metadataStore,
	}
}

// GeneratePreview creates a complete order preview using Prime REST API
func (s *OrderService) GeneratePreview(ctx context.Context, req OrderRequest) (*OrderPreviewResponse, error) {
	// Validate request
	if err := ValidateOrderRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Prepare order request with fee calculations
	prepared, err := PrepareOrderRequest(req, s.portfolioId, s.priceAdjuster, false)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare order: %w", err)
	}

	// Log preview details for quote-denominated orders
	if prepared.Metadata != nil {
		zap.L().Info("Order Preview requested",
			zap.String("requested_amount", prepared.Metadata.UserRequestedAmount.String()),
			zap.String("markup_amount", prepared.Metadata.MarkupAmount.String()),
			zap.String("prime_preview_amount", prepared.Metadata.PrimeOrderAmount.String()))
	}

	// Add timeout to API call (10 seconds for preview)
	apiCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Call Prime REST API for order preview
	primeResp, err := s.ordersSvc.CreateOrderPreview(apiCtx, prepared.PrimeRequest)
	if err != nil {
		return nil, fmt.Errorf("Prime API error: %w", err)
	}

	// Parse Prime's response
	order := primeResp.Order
	executionPrice, err := decimal.NewFromString(order.AverageFilledPrice)
	if err != nil {
		return nil, fmt.Errorf("failed to parse execution price: %w", err)
	}

	primeFee, err := decimal.NewFromString(order.Commission)
	if err != nil {
		return nil, fmt.Errorf("failed to parse commission: %w", err)
	}

	// Get the actual base quantity (important for quote-denominated orders)
	// The Prime API returns the base quantity regardless of how we specified it
	baseQty, err := decimal.NewFromString(order.BaseQuantity)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base quantity: %w", err)
	}

	// Build raw Prime preview (what Prime returns)
	rawPreview := &RawPrimePreview{
		Quantity:           RoundQty(baseQty),
		AverageFilledPrice: RoundPrice(executionPrice),
		TotalValue:         RoundPrice(decimal.RequireFromString(order.Total)),
		Commission:         RoundPrice(primeFee),
	}

	// Calculate custom fee (our markup) on top of Prime's execution
	customFee := s.priceAdjuster.ComputeFee(baseQty, executionPrice)

	// Calculate effective price (total cost / quantity including our fee)
	totalCost := baseQty.Mul(executionPrice).Add(primeFee).Add(customFee)
	effectivePrice := decimal.Zero
	if !baseQty.IsZero() {
		effectivePrice = totalCost.Div(baseQty)
	}

	// Build custom fee overlay (our calculations)
	var customOverlay *CustomFeeOverlay
	if !customFee.IsZero() {
		notional := baseQty.Mul(executionPrice)
		feePercent := decimal.Zero
		if !notional.IsZero() {
			feePercent = customFee.Div(notional).Mul(decimal.NewFromInt(100))
		}
		customOverlay = &CustomFeeOverlay{
			FeeAmount:      RoundPrice(customFee),
			FeePercent:     feePercent.Round(2).String(),
			EffectivePrice: RoundPrice(effectivePrice),
		}
	}

	// Build response
	response := &OrderPreviewResponse{
		Product:          req.Product,
		Side:             req.Side,
		Type:             req.Type,
		OrderUnit:        req.Unit, // "base" or "quote"
		RawPreview:       rawPreview,
		CustomFeeOverlay: customOverlay,
		Timestamp:        time.Now(),
	}

	// Add quote order specific fields
	if prepared.Metadata != nil {
		response.UserRequestedAmount = prepared.Metadata.UserRequestedAmount.String()
	}

	// Add limit order price
	if !req.Price.IsZero() {
		response.RequestedPrice = req.Price.String()
	}

	return response, nil
}

// PlaceOrder places an actual order with Prime and returns immediately
// Order updates should be tracked via the orders websocket
// IMPORTANT: For quote-denominated orders, we deduct our markup BEFORE sending to Prime
func (s *OrderService) PlaceOrder(ctx context.Context, req OrderRequest) (*OrderResponse, error) {
	// Validate request
	if err := ValidateOrderRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Prepare order request with fee calculations (generate client order Id for actual orders)
	prepared, err := PrepareOrderRequest(req, s.portfolioId, s.priceAdjuster, true)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare order: %w", err)
	}

	// Log order details for quote-denominated orders
	if prepared.Metadata != nil {
		zap.L().Info("Adjusted order for custom markup",
			zap.String("requested_amount", prepared.Metadata.UserRequestedAmount.String()),
			zap.String("custom_fee", prepared.Metadata.MarkupAmount.String()),
			zap.String("prime_order_amount", prepared.Metadata.PrimeOrderAmount.String()))
	}

	// Add timeout to API call (15 seconds for actual order placement)
	apiCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Call Prime REST API to place the order
	createResp, err := s.ordersSvc.CreateOrder(apiCtx, prepared.PrimeRequest)
	if err != nil {
		return nil, fmt.Errorf("Prime API error: %w", err)
	}

	// Store metadata using the order Id from Prime (for websocket handler to retrieve)
	if s.metadataStore != nil && prepared.Metadata != nil {
		s.metadataStore.Set(createResp.OrderId, map[string]decimal.Decimal{
			"UserRequestedAmount": prepared.Metadata.UserRequestedAmount,
			"MarkupAmount":        prepared.Metadata.MarkupAmount,
			"PrimeOrderAmount":    prepared.Metadata.PrimeOrderAmount,
		})
	}

	// Return minimal response - websocket will handle updates
	response := &OrderResponse{
		OrderId:       createResp.OrderId,
		ClientOrderId: prepared.NormalizedReq.ClientOrderId,
		Product:       req.Product,
		Side:          prepared.NormalizedReq.Side,
		Type:          prepared.NormalizedReq.Type,
		Status:        "PENDING", // Order submitted, waiting for websocket updates
		Timestamp:     time.Now(),
	}

	return response, nil
}
