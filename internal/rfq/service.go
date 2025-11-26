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

package rfq

import (
	"context"
	"fmt"
	"time"

	"github.com/coinbase-samples/prime-sdk-go/model"
	"github.com/coinbase-samples/prime-sdk-go/orders"
	"github.com/coinbase-samples/prime-trading-fees-go/config"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/common"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/fees"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type RfqService struct {
	primeClient   orders.OrdersService
	portfolioId   string
	priceAdjuster *fees.PriceAdjuster
}

// NewRfqService creates a new RFQ service
func NewRfqService(cfg *config.Config, priceAdjuster *fees.PriceAdjuster, primeClient orders.OrdersService) *RfqService {
	return &RfqService{
		primeClient:   primeClient,
		portfolioId:   cfg.Prime.Portfolio,
		priceAdjuster: priceAdjuster,
	}
}

// CreateQuote creates an RFQ quote with fee markup applied
func (s *RfqService) CreateQuote(ctx context.Context, req common.RfqRequest) (*common.RfqResponse, error) {
	// Validate request
	if err := common.ValidateRfqRequest(req); err != nil {
		return nil, err
	}

	// Build Prime RFQ request with fee adjustments
	primeReq, originalAmount, feeAmount := s.buildPrimeQuoteRequest(req)

	// Call Prime API
	primeResp, err := s.primeClient.CreateQuoteRequest(ctx, primeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create quote: %w", err)
	}

	// Build response with fee overlay
	response := s.buildQuoteResponse(primeResp, req, originalAmount, feeAmount)

	zap.L().Info("RFQ quote created",
		zap.String("quote_id", response.QuoteId),
		zap.String("product", response.Product),
		zap.String("side", response.Side),
		zap.String("user_amount", response.UserRequestedAmount),
		zap.String("fee", response.CustomFeeOverlay.FeeAmount))

	return response, nil
}

// AcceptQuote accepts an RFQ quote
func (s *RfqService) AcceptQuote(ctx context.Context, req common.AcceptRfqRequest) (*common.AcceptRfqResponse, error) {
	// Generate client order ID if not provided
	clientOrderId := req.ClientOrderId
	if clientOrderId == "" {
		clientOrderId = uuid.New().String()
	}

	primeReq := &orders.AcceptQuoteRequest{
		PortfolioId:   s.portfolioId,
		ProductId:     req.Product,
		Side:          req.Side,
		ClientOrderId: clientOrderId,
		QuoteId:       req.QuoteId,
	}

	primeResp, err := s.primeClient.AcceptQuote(ctx, primeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to accept quote: %w", err)
	}

	response := &common.AcceptRfqResponse{
		OrderId:       primeResp.OrderId,
		QuoteId:       req.QuoteId,
		ClientOrderId: clientOrderId,
		Product:       req.Product,
		Side:          req.Side,
	}

	zap.L().Info("RFQ quote accepted",
		zap.String("order_id", response.OrderId),
		zap.String("quote_id", response.QuoteId),
		zap.String("product", response.Product))

	return response, nil
}

// buildPrimeQuoteRequest builds the Prime API request with fee adjustments
func (s *RfqService) buildPrimeQuoteRequest(req common.RfqRequest) (*orders.CreateQuoteRequest, decimal.Decimal, decimal.Decimal) {
	primeReq := &orders.CreateQuoteRequest{
		PortfolioId:   s.portfolioId,
		ProductId:     req.Product,
		Side:          model.OrderSide(req.Side),
		ClientQuoteId: uuid.New().String(),
	}

	var originalAmount, feeAmount decimal.Decimal

	// Limit price is always required for RFQ
	primeReq.LimitPrice = req.LimitPrice.String()

	if req.Unit == "quote" {
		// Quote-denominated: Hold fee upfront, send reduced amount to Prime
		originalAmount = req.QuoteValue
		feeAmount = s.priceAdjuster.FeeStrategy.ComputeFromNotional(req.QuoteValue)
		primeAmount := req.QuoteValue.Sub(feeAmount)
		primeReq.QuoteValue = primeAmount.String()
	} else {
		// Base-denominated: Send full quantity to Prime, fee added on top later
		originalAmount = req.BaseQty
		feeAmount = decimal.Zero // Will be calculated after Prime responds
		primeReq.BaseQuantity = req.BaseQty.String()
	}

	return primeReq, originalAmount, feeAmount
}

// buildQuoteResponse builds the user-facing response with fee overlay
func (s *RfqService) buildQuoteResponse(primeResp *orders.CreateQuoteResponse, req common.RfqRequest, originalAmount, feeAmount decimal.Decimal) *common.RfqResponse {
	response := &common.RfqResponse{
		QuoteId:             primeResp.QuoteId,
		Product:             req.Product,
		Side:                req.Side,
		ExpirationTime:      primeResp.ExpirationTime,
		Unit:                req.Unit,
		UserRequestedAmount: originalAmount.String(),
		Timestamp:           time.Now().UTC().Format(time.RFC3339),
	}

	// Set raw Prime quote data
	response.RawPrimeQuote.BestPrice = primeResp.BestPrice
	response.RawPrimeQuote.OrderTotal = primeResp.OrderTotal
	response.RawPrimeQuote.PriceInclusiveOfFees = primeResp.PriceInclusiveOfFees

	// Calculate fee overlay
	feePercent := s.priceAdjuster.FeeStrategy.Percent.Mul(decimal.NewFromInt(100))

	if req.Unit == "quote" {
		// Quote orders: fee already held upfront
		response.CustomFeeOverlay.FeeAmount = feeAmount.String()
		response.CustomFeeOverlay.FeePercent = feePercent.String()

		// Calculate effective price
		primeTotal, _ := decimal.NewFromString(primeResp.OrderTotal)
		if !primeTotal.IsZero() {
			// Effective price = (user paid + fee) / quantity
			totalCost := originalAmount
			response.CustomFeeOverlay.TotalCost = totalCost.String()

			// Parse best price to get implied quantity
			bestPrice, _ := decimal.NewFromString(primeResp.BestPrice)
			if !bestPrice.IsZero() {
				qty := primeTotal.Div(bestPrice)
				effectivePrice := totalCost.Div(qty)
				response.CustomFeeOverlay.EffectivePrice = effectivePrice.StringFixed(2)
			}
		}
	} else {
		// Base orders: calculate fee on top of Prime's cost
		primeTotal, _ := decimal.NewFromString(primeResp.OrderTotal)
		feeAmount = s.priceAdjuster.FeeStrategy.ComputeFromNotional(primeTotal)
		totalCost := primeTotal.Add(feeAmount)

		response.CustomFeeOverlay.FeeAmount = feeAmount.String()
		response.CustomFeeOverlay.FeePercent = feePercent.String()
		response.CustomFeeOverlay.TotalCost = totalCost.String()

		// Effective price = total cost / base quantity
		if !originalAmount.IsZero() {
			effectivePrice := totalCost.Div(originalAmount)
			response.CustomFeeOverlay.EffectivePrice = effectivePrice.StringFixed(2)
		}
	}

	return response
}
