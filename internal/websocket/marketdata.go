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

package websocket

import (
	"fmt"
	"sort"
	"time"

	"github.com/coinbase-samples/prime-trading-fees-go/internal/common"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// MarketDataConfig holds configuration for the Prime WebSocket connection
type MarketDataConfig struct {
	Url              string
	AccessKey        string
	Passphrase       string
	SigningKey       string
	ServiceAccountId string
	Portfolio        string
	Products         []string
	MaxLevels        int
	ReconnectDelay   time.Duration
}

// MarketDataClient manages the connection to Coinbase Prime WebSocket
type MarketDataClient struct {
	config     MarketDataConfig
	store      *OrderBookStore
	baseClient *BaseWebSocketClient
}

// NewMarketDataClient creates a new WebSocket client
func NewMarketDataClient(config MarketDataConfig, store *OrderBookStore) *MarketDataClient {
	client := &MarketDataClient{
		config: config,
		store:  store,
	}

	baseConfig := BaseConfig{
		Url:              config.Url,
		AccessKey:        config.AccessKey,
		Passphrase:       config.Passphrase,
		SigningKey:       config.SigningKey,
		ServiceAccountId: config.ServiceAccountId,
		ReconnectDelay:   config.ReconnectDelay,
	}

	client.baseClient = NewBaseWebSocketClient(baseConfig, client)
	return client
}

// Start begins the WebSocket connection and message processing
func (c *MarketDataClient) Start() error {
	return c.baseClient.Start()
}

// Stop gracefully stops the WebSocket client
func (c *MarketDataClient) Stop() {
	c.baseClient.Stop()
}

// ChannelHandler interface implementation

// GetChannelName returns the channel name for this handler
func (c *MarketDataClient) GetChannelName() string {
	return "l2_data"
}

// BuildSignatureMessage builds the message string to be signed
func (c *MarketDataClient) BuildSignatureMessage(baseConfig BaseConfig, timestamp string) string {
	// Concatenate all product IDs for signature (e.g., "BTC-USDETH-USD")
	productIdsJoined := ""
	for _, p := range c.config.Products {
		productIdsJoined += p
	}

	// Format: channel + accessKey + serviceAccountId + timestamp + joinedProductIDs
	return c.GetChannelName() + baseConfig.AccessKey + baseConfig.ServiceAccountId + timestamp + productIdsJoined
}

// BuildSubscriptionMessage builds the subscription payload
func (c *MarketDataClient) BuildSubscriptionMessage(baseConfig BaseConfig, timestamp string, signature string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "subscribe",
		"channel":     c.GetChannelName(),
		"access_key":  baseConfig.AccessKey,
		"api_key_id":  baseConfig.ServiceAccountId,
		"timestamp":   timestamp,
		"passphrase":  baseConfig.Passphrase,
		"signature":   signature,
		"product_ids": c.config.Products,
	}
}

// HandleMessage processes messages for the l2_data channel
func (c *MarketDataClient) HandleMessage(message map[string]interface{}) error {
	// Get events array
	eventsRaw, ok := message["events"].([]interface{})
	if !ok || len(eventsRaw) == 0 {
		return fmt.Errorf("message missing events array")
	}

	// Process each event
	for _, eventRaw := range eventsRaw {
		event, ok := eventRaw.(map[string]interface{})
		if !ok {
			continue
		}

		if err := c.handleL2Event(event); err != nil {
			zap.L().Error("Error handling L2 event", zap.Error(err))
		}
	}

	return nil
}

func (c *MarketDataClient) handleL2Event(event map[string]interface{}) error {
	// Parse event metadata
	eventType, productId, err := c.parseEventMetadata(event)
	if err != nil {
		return err
	}

	// Parse price level updates
	updates, err := c.parseUpdates(event)
	if err != nil {
		return err
	}

	// Get or create order book
	book := c.store.GetOrCreate(productId)

	// Apply updates based on event type
	if eventType == "snapshot" {
		return c.handleSnapshot(book, updates)
	}
	return c.handleUpdate(book, updates)
}

// parseEventMetadata extracts event type and product Id from event
func (c *MarketDataClient) parseEventMetadata(event map[string]interface{}) (string, string, error) {
	eventType, ok := event["type"].(string)
	if !ok {
		return "", "", fmt.Errorf("event missing type field")
	}

	productId, ok := event["product_id"].(string)
	if !ok || productId == "" {
		return "", "", fmt.Errorf("event missing product_id")
	}

	return eventType, productId, nil
}

// parseUpdates extracts and parses all price level updates from an event
func (c *MarketDataClient) parseUpdates(event map[string]interface{}) (map[string]common.PriceLevel, error) {
	updatesRaw, ok := event["updates"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("event missing updates array")
	}

	levels := make(map[string]common.PriceLevel)

	for _, updateRaw := range updatesRaw {
		update, ok := updateRaw.(map[string]interface{})
		if !ok {
			continue
		}

		side, _ := update["side"].(string)

		// Parse price and size using decimal (never float64 for financial data)
		price, err := c.parseDecimalField(update, "px")
		if err != nil {
			continue
		}

		size, err := c.parseDecimalField(update, "qty")
		if err != nil {
			continue
		}

		priceLevel := common.PriceLevel{
			Price: price,
			Size:  size,
		}

		key := side + ":" + priceLevel.Price.String()
		levels[key] = priceLevel
	}

	return levels, nil
}

// parseDecimalField safely parses a field that may be string or float64 into decimal.Decimal
// This handles JSON variability while ensuring we always use precise decimal types for financial data
func (c *MarketDataClient) parseDecimalField(update map[string]interface{}, key string) (decimal.Decimal, error) {
	val, ok := update[key]
	if !ok {
		return decimal.Zero, fmt.Errorf("missing field: %s", key)
	}

	switch v := val.(type) {
	case string:
		return decimal.NewFromString(v)
	case float64:
		// Convert float64 from JSON to decimal immediately
		// We never keep financial data as float64
		return decimal.NewFromFloat(v), nil
	default:
		return decimal.Zero, fmt.Errorf("invalid type for %s", key)
	}
}

// handleSnapshot replaces the entire order book with snapshot data
func (c *MarketDataClient) handleSnapshot(book *OrderBook, levels map[string]common.PriceLevel) error {
	bids, asks := c.buildOrderBook(levels)
	book.Update(bids, asks, 0)
	return nil
}

// handleUpdate applies incremental updates to existing order book
func (c *MarketDataClient) handleUpdate(book *OrderBook, newLevels map[string]common.PriceLevel) error {
	snapshot := book.Snapshot()

	// Build maps of existing levels
	bidMap := make(map[string]common.PriceLevel)
	askMap := make(map[string]common.PriceLevel)

	for _, bid := range snapshot.Bids {
		bidMap[bid.Price.String()] = bid
	}
	for _, ask := range snapshot.Asks {
		askMap[ask.Price.String()] = ask
	}

	// Apply updates (replace, add, or remove)
	for key, level := range newLevels {
		priceKey := level.Price.String()
		if key[:3] == "bid" {
			if level.Size.IsZero() {
				delete(bidMap, priceKey)
			} else {
				bidMap[priceKey] = level
			}
		} else if key[:3] == "ask" {
			if level.Size.IsZero() {
				delete(askMap, priceKey)
			} else {
				askMap[priceKey] = level
			}
		}
	}

	// Convert maps back to slices and sort
	bids := c.mapToSlice(bidMap)
	asks := c.mapToSlice(askMap)

	c.sortBids(bids)
	c.sortAsks(asks)

	bids = c.limitLevels(bids)
	asks = c.limitLevels(asks)

	book.Update(bids, asks, 0)
	return nil
}

// buildOrderBook converts a map of levels into sorted, limited bid/ask slices
func (c *MarketDataClient) buildOrderBook(levels map[string]common.PriceLevel) ([]common.PriceLevel, []common.PriceLevel) {
	bids := []common.PriceLevel{}
	asks := []common.PriceLevel{}

	for key, level := range levels {
		if level.Size.IsZero() {
			continue
		}
		if key[:3] == "bid" {
			bids = append(bids, level)
		} else if key[:3] == "ask" || key[:3] == "off" { // Prime might use "offer"
			asks = append(asks, level)
		}
	}

	c.sortBids(bids)
	c.sortAsks(asks)

	return c.limitLevels(bids), c.limitLevels(asks)
}

// mapToSlice converts a map of price levels to a slice
func (c *MarketDataClient) mapToSlice(m map[string]common.PriceLevel) []common.PriceLevel {
	result := make([]common.PriceLevel, 0, len(m))
	for _, level := range m {
		result = append(result, level)
	}
	return result
}

// limitLevels caps the number of levels if MaxLevels is configured
func (c *MarketDataClient) limitLevels(levels []common.PriceLevel) []common.PriceLevel {
	if c.config.MaxLevels > 0 && len(levels) > c.config.MaxLevels {
		return levels[:c.config.MaxLevels]
	}
	return levels
}

// sortBids sorts bids in descending order (highest price first)
func (c *MarketDataClient) sortBids(bids []common.PriceLevel) {
	sort.Slice(bids, func(i, j int) bool {
		return bids[i].Price.GreaterThan(bids[j].Price)
	})
}

// sortAsks sorts asks in ascending order (lowest price first)
func (c *MarketDataClient) sortAsks(asks []common.PriceLevel) {
	sort.Slice(asks, func(i, j int) bool {
		return asks[i].Price.LessThan(asks[j].Price)
	})
}
