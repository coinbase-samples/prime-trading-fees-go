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

package orders

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/coinbase-samples/prime-trading-fees-go/internal/database"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/fees"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/order"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// MetadataStore stores order metadata in memory
type MetadataStore struct {
	mu   sync.RWMutex
	data map[string]interface{} // key: order_id, value: *order.OrderMetadata or map
}

// NewMetadataStore creates a new metadata store
func NewMetadataStore() *MetadataStore {
	return &MetadataStore{
		data: make(map[string]interface{}),
	}
}

// Set stores metadata for an order
func (s *MetadataStore) Set(orderId string, metadata interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[orderId] = metadata
}

// Get retrieves metadata for an order (returns interface{} to support different types)
func (s *MetadataStore) Get(orderId string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	metadata, ok := s.data[orderId]
	return metadata, ok
}

// Delete removes metadata for an order (cleanup after final state)
func (s *MetadataStore) Delete(orderId string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, orderId)
}

// DbOrderHandler processes order updates and stores them in the database
type DbOrderHandler struct {
	db            *database.OrdersDb
	priceAdjuster *fees.PriceAdjuster
	metadataStore *MetadataStore
}

// NewDbOrderHandler creates a new database order handler
func NewDbOrderHandler(db *database.OrdersDb, priceAdjuster *fees.PriceAdjuster, metadataStore *MetadataStore) *DbOrderHandler {
	return &DbOrderHandler{
		db:            db,
		priceAdjuster: priceAdjuster,
		metadataStore: metadataStore,
	}
}

// HandleOrderUpdate processes a websocket order update message
func (h *DbOrderHandler) HandleOrderUpdate(update map[string]interface{}) error {
	// Extract sequence number and timestamp
	sequenceNum, ok := update["sequence_num"].(float64)
	if !ok {
		zap.L().Warn("Missing or invalid sequence_num in order update")
		sequenceNum = 0
	}

	timestampStr, ok := update["timestamp"].(string)
	if !ok {
		zap.L().Warn("Missing or invalid timestamp in order update")
		timestampStr = ""
	}

	timestamp, err := time.Parse(time.RFC3339Nano, timestampStr)
	if err != nil || timestamp.IsZero() {
		timestamp = time.Now()
	}

	// Get events array
	eventsRaw, ok := update["events"].([]interface{})
	if !ok || len(eventsRaw) == 0 {
		return nil
	}

	// Process each event
	for _, eventRaw := range eventsRaw {
		event, ok := eventRaw.(map[string]interface{})
		if !ok {
			continue
		}

		eventType, ok := event["type"].(string)
		if !ok {
			zap.L().Warn("Missing or invalid event type")
			eventType = "unknown"
		}

		// Get orders array from event
		ordersRaw, ok := event["orders"].([]interface{})
		if !ok || len(ordersRaw) == 0 {
			continue
		}

		// Process each order in the event
		for _, orderRaw := range ordersRaw {
			orderData, ok := orderRaw.(map[string]interface{})
			if !ok {
				continue
			}

			if err := h.processOrderUpdate(orderData, eventType, int64(sequenceNum), timestamp); err != nil {
				zap.L().Error("Failed to process order update",
					zap.String("order_id", getString(orderData, "order_id")),
					zap.Error(err))
			}
		}
	}

	return nil
}

func (h *DbOrderHandler) processOrderUpdate(orderData map[string]interface{}, eventType string, sequenceNum int64, timestamp time.Time) error {
	orderId := getString(orderData, "order_id")
	if orderId == "" {
		return fmt.Errorf("missing order_id")
	}

	// Parse order fields
	clientOrderId := getString(orderData, "client_order_id")
	productId := getString(orderData, "product_id")
	side := getString(orderData, "side")
	orderType := getString(orderData, "order_type")
	status := getString(orderData, "status")
	cumQty := getString(orderData, "cum_qty")
	leavesQty := getString(orderData, "leaves_qty")
	avgPx := getString(orderData, "avg_px")
	netAvgPx := getString(orderData, "net_avg_px")
	feesStr := getString(orderData, "fees")
	commission := getString(orderData, "commission")
	venueFee := getString(orderData, "venue_fee")
	cesCommission := getString(orderData, "ces_commission")

	// Convert to JSON for event storage
	rawJSON, err := json.Marshal(orderData)
	if err != nil {
		return fmt.Errorf("failed to marshal order data: %w", err)
	}

	// Insert event into audit log
	event := &database.OrderEvent{
		OrderId:       orderId,
		SequenceNum:   sequenceNum,
		EventType:     eventType,
		Status:        status,
		CumQty:        cumQty,
		LeavesQty:     leavesQty,
		AvgPx:         avgPx,
		NetAvgPx:      netAvgPx,
		Fees:          feesStr,
		Commission:    commission,
		VenueFee:      venueFee,
		CesCommission: cesCommission,
		RawJson:       string(rawJSON),
		ReceivedAt:    timestamp,
	}

	if err := h.db.InsertOrderEvent(event); err != nil {
		return fmt.Errorf("failed to insert order event: %w", err)
	}

	// Check if this is the first time we've seen this order
	existing, err := h.db.GetOrder(orderId)
	if err != nil {
		return fmt.Errorf("failed to check existing order: %w", err)
	}

	firstSeenAt := timestamp
	if existing != nil {
		firstSeenAt = existing.FirstSeenAt
	}

	// Get order metadata (upfront amounts)
	// First try in-memory store, then fallback to database
	metadataRaw, hasMetadata := h.metadataStore.Get(orderId)
	userRequestedAmount := "0"
	MarkupAmount := "0"
	primeOrderAmount := "0"

	if hasMetadata {
		// Handle typed metadata from in-memory store
		if meta, ok := metadataRaw.(*order.OrderMetadata); ok {
			userRequestedAmount = meta.UserRequestedAmount.String()
			MarkupAmount = meta.MarkupAmount.String()
			primeOrderAmount = meta.PrimeOrderAmount.String()
		} else if metaMap, ok := metadataRaw.(map[string]decimal.Decimal); ok {
			// Handle map-based metadata (from order placement)
			if val, ok := metaMap["UserRequestedAmount"]; ok {
				userRequestedAmount = val.String()
			}
			if val, ok := metaMap["MarkupAmount"]; ok {
				MarkupAmount = val.String()
			}
			if val, ok := metaMap["PrimeOrderAmount"]; ok {
				primeOrderAmount = val.String()
			}
		}
	} else if existing != nil {
		// Fallback to database metadata (for orders placed by separate process)
		userRequestedAmount = existing.UserRequestedAmount
		MarkupAmount = existing.MarkupAmount
		primeOrderAmount = existing.PrimeOrderAmount
	}

	// Calculate fee settlement for terminal states (quote-denominated orders only)
	actualFilledValue := "0"
	actualEarnedFee := "0"
	rebateAmount := "0"
	feeSettled := false

	isTerminal := (status == "FILLED" || status == "CANCELLED" || status == "REJECTED")
	if isTerminal && MarkupAmount != "0" && MarkupAmount != "" {
		settlement := h.calculateFeeSettlement(cumQty, avgPx, userRequestedAmount, MarkupAmount, primeOrderAmount)
		actualFilledValue = settlement.ActualFilledValue
		actualEarnedFee = settlement.ActualEarnedFee
		rebateAmount = settlement.RebateAmount
		feeSettled = true

		// Log the settlement
		if settlement.RebateAmount != "0" && settlement.RebateAmount != "" {
			zap.L().Info("Fee settlement calculated",
				zap.String("order_id", orderId[:8]+"..."),
				zap.String("status", status),
				zap.String("filled_value", actualFilledValue),
				zap.String("earned_fee", actualEarnedFee),
				zap.String("rebate", rebateAmount))
		}
	}

	// Upsert order record (UPDATE if exists, INSERT if new)
	// Round values to reasonable precision before storing
	orderRecord := &database.OrderRecord{
		OrderId:             orderId,
		ClientOrderId:       clientOrderId,
		ProductId:           productId,
		Side:                side,
		OrderType:           orderType,
		Status:              status,
		CumQty:              roundQtyString(cumQty),
		LeavesQty:           roundQtyString(leavesQty),
		AvgPx:               roundPriceString(avgPx),
		NetAvgPx:            roundPriceString(netAvgPx),
		Fees:                roundPriceString(feesStr),
		Commission:          roundPriceString(commission),
		VenueFee:            roundPriceString(venueFee),
		CesCommission:       roundPriceString(cesCommission),
		UserRequestedAmount: userRequestedAmount,
		MarkupAmount:        MarkupAmount,
		PrimeOrderAmount:    primeOrderAmount,
		ActualFilledValue:   actualFilledValue,
		ActualEarnedFee:     actualEarnedFee,
		RebateAmount:        rebateAmount,
		FeeSettled:          feeSettled,
		FirstSeenAt:         firstSeenAt,
		LastUpdatedAt:       timestamp,
	}

	if err := h.db.UpsertOrder(orderRecord); err != nil {
		return fmt.Errorf("failed to upsert order: %w", err)
	}

	// Clean up metadata for terminal states
	if status == "FILLED" || status == "CANCELLED" || status == "REJECTED" {
		h.metadataStore.Delete(orderId)
	}

	// Log the update - for terminal states, fills, or status transitions to OPEN
	cumQtyDec, _ := decimal.NewFromString(cumQty)
	statusChanged := (existing == nil || existing.Status != status)
	isTerminalOrFilled := status == "FILLED" || status == "CANCELLED" || status == "REJECTED" || !cumQtyDec.IsZero()
	isOpenTransition := (status == "OPEN" && statusChanged)

	if isTerminalOrFilled || isOpenTransition {
		zap.L().Info("Order event",
			zap.String("order_id", orderId[:8]+"..."), // Truncate for readability
			zap.String("client_order_id", clientOrderId[:8]+"..."),
			zap.String("status", status),
			zap.String("filled", cumQty),
			zap.String("price", avgPx))
	}

	return nil
}

// Helper function to safely extract string from map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// roundPriceString rounds a price string to 2 decimal places
func roundPriceString(s string) string {
	if s == "" || s == "0" {
		return s
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return s
	}
	return d.Round(2).String()
}

// roundQtyString rounds a quantity string to 8 decimal places
func roundQtyString(s string) string {
	if s == "" || s == "0" {
		return s
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return s
	}
	return d.Round(8).String()
}

// Note: We keep these string-based rounding functions here because they handle empty strings
// and errors gracefully for websocket data. The order.RoundPrice/RoundQty work on decimals directly.

// FeeSettlement represents the calculated fee settlement for a terminal order
type FeeSettlement struct {
	ActualFilledValue string
	ActualEarnedFee   string
	RebateAmount      string
}

// calculateFeeSettlement calculates the actual fee earned and rebate amount
// based on the actual filled amount vs the upfront fee hold.
//
// Example: User requested $10, we held $0.05 fee (50 bps), sent $9.95 to Prime
// - 100% fill: Prime filled $9.95, earned fee = $0.05, rebate = $0
// - 50% fill: Prime filled $4.975, earned fee = $0.025, rebate = $0.025
// - 0% fill: Prime filled $0, earned fee = $0, rebate = $0.05
//
// Math:
// - fee_rate = markup / user_requested
// - actual_filled_value = cum_qty * avg_px (what Prime actually filled)
// - actual_user_cost = actual_filled_value / (1 - fee_rate) (what user should pay including fee)
// - actual_earned_fee = actual_user_cost * fee_rate
// - rebate = markup - actual_earned_fee
func (h *DbOrderHandler) calculateFeeSettlement(cumQty, avgPx, userRequestedAmount, markupAmount, primeOrderAmount string) FeeSettlement {
	// Parse inputs
	cumQtyDec, err := decimal.NewFromString(cumQty)
	if err != nil || cumQtyDec.IsZero() {
		// No fill - full rebate
		return FeeSettlement{
			ActualFilledValue: "0",
			ActualEarnedFee:   "0",
			RebateAmount:      markupAmount,
		}
	}

	avgPxDec, err := decimal.NewFromString(avgPx)
	if err != nil || avgPxDec.IsZero() {
		// No fill - full rebate
		return FeeSettlement{
			ActualFilledValue: "0",
			ActualEarnedFee:   "0",
			RebateAmount:      markupAmount,
		}
	}

	markupAmountDec, err := decimal.NewFromString(markupAmount)
	if err != nil || markupAmountDec.IsZero() {
		// No markup - no settlement needed
		return FeeSettlement{
			ActualFilledValue: roundPriceString(cumQtyDec.Mul(avgPxDec).String()),
			ActualEarnedFee:   "0",
			RebateAmount:      "0",
		}
	}

	userRequestedDec, err := decimal.NewFromString(userRequestedAmount)
	if err != nil || userRequestedDec.IsZero() {
		// Can't calculate without user requested amount - be conservative and rebate nothing
		actualFilledValue := cumQtyDec.Mul(avgPxDec)
		return FeeSettlement{
			ActualFilledValue: roundPriceString(actualFilledValue.String()),
			ActualEarnedFee:   roundPriceString(markupAmountDec.String()),
			RebateAmount:      "0",
		}
	}

	// Calculate actual filled notional value: cum_qty * avg_px
	actualFilledValue := cumQtyDec.Mul(avgPxDec)

	// Calculate the fee rate from the original order
	// fee_rate = markup / user_requested
	feeRate := markupAmountDec.Div(userRequestedDec)

	// Calculate what the user should actually pay (including our fee)
	// actual_user_cost = actual_filled_value / (1 - fee_rate)
	oneMinusFeeRate := decimal.NewFromInt(1).Sub(feeRate)
	if oneMinusFeeRate.LessThanOrEqual(decimal.Zero) {
		// Invalid fee rate - shouldn't happen
		return FeeSettlement{
			ActualFilledValue: roundPriceString(actualFilledValue.String()),
			ActualEarnedFee:   "0",
			RebateAmount:      roundPriceString(markupAmountDec.String()),
		}
	}

	actualUserCost := actualFilledValue.Div(oneMinusFeeRate)

	// Calculate the actual fee we earned
	// actual_earned_fee = actual_user_cost * fee_rate
	actualEarnedFee := actualUserCost.Mul(feeRate)

	// Cap earned fee at markup amount (can't earn more than we held)
	if actualEarnedFee.GreaterThan(markupAmountDec) {
		actualEarnedFee = markupAmountDec
	}

	// Calculate rebate
	rebateAmount := markupAmountDec.Sub(actualEarnedFee)
	if rebateAmount.IsNegative() {
		rebateAmount = decimal.Zero
	}

	return FeeSettlement{
		ActualFilledValue: roundPriceString(actualFilledValue.String()),
		ActualEarnedFee:   roundPriceString(actualEarnedFee.String()),
		RebateAmount:      roundPriceString(rebateAmount.String()),
	}
}
