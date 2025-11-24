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

package marketdata

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// WebSocketConfig holds configuration for the Prime WebSocket connection
type WebSocketConfig struct {
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

// WebSocketClient manages the connection to Coinbase Prime WebSocket
type WebSocketClient struct {
	config    WebSocketConfig
	store     *OrderBookStore
	conn      *websocket.Conn
	ctx       context.Context
	cancel    context.CancelFunc
	reconnect bool
}

// NewWebSocketClient creates a new WebSocket client
func NewWebSocketClient(config WebSocketConfig, store *OrderBookStore) *WebSocketClient {
	ctx, cancel := context.WithCancel(context.Background())

	return &WebSocketClient{
		config:    config,
		store:     store,
		ctx:       ctx,
		cancel:    cancel,
		reconnect: true,
	}
}

// Start begins the WebSocket connection and message processing
func (c *WebSocketClient) Start() error {
	go c.run()
	return nil
}

// Stop gracefully stops the WebSocket client
func (c *WebSocketClient) Stop() {
	c.reconnect = false
	c.cancel()
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *WebSocketClient) run() {
	for c.reconnect {
		if err := c.connect(); err != nil {
			zap.L().Error("Failed to connect", zap.Error(err))
			time.Sleep(c.config.ReconnectDelay)
			continue
		}

		if err := c.subscribe(); err != nil {
			zap.L().Error("Failed to subscribe", zap.Error(err))
			c.conn.Close()
			time.Sleep(c.config.ReconnectDelay)
			continue
		}

		c.readMessages()

		// Connection closed, reconnect if enabled
		if c.reconnect {
			zap.L().Info("Reconnecting", zap.Duration("delay", c.config.ReconnectDelay))
			time.Sleep(c.config.ReconnectDelay)
		}
	}
}

func (c *WebSocketClient) connect() error {
	zap.L().Info("Connecting to Prime WebSocket", zap.String("url", c.config.Url))

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(c.config.Url, nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	c.conn = conn
	zap.L().Info("Connected to Prime WebSocket")
	return nil
}

func (c *WebSocketClient) subscribe() error {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	channel := "l2_data"

	// Concatenate all product IDs for signature (e.g., "BTC-USDETH-USD")
	productIdsJoined := ""
	for _, p := range c.config.Products {
		productIdsJoined += p
	}

	// Create signature for WebSocket authentication
	// Format: channel + accessKey + serviceAccountId + timestamp + joinedProductIDs
	message := channel + c.config.AccessKey + c.config.ServiceAccountId + timestamp + productIdsJoined
	signature := c.sign(message)

	// Build subscription message
	sub := map[string]interface{}{
		"type":        "subscribe",
		"channel":     channel,
		"access_key":  c.config.AccessKey,
		"api_key_id":  c.config.ServiceAccountId, // Required for WebSocket auth
		"timestamp":   timestamp,
		"passphrase":  c.config.Passphrase,
		"signature":   signature,
		"product_ids": c.config.Products,
	}

	if err := c.conn.WriteJSON(sub); err != nil {
		return fmt.Errorf("failed to send subscription: %w", err)
	}

	zap.L().Info("Sent subscription request", zap.Strings("products", c.config.Products))
	return nil
}

func (c *WebSocketClient) sign(message string) string {
	h := hmac.New(sha256.New, []byte(c.config.SigningKey))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (c *WebSocketClient) readMessages() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		_, message, err := c.conn.ReadMessage()
		if err != nil {
			// Don't log error if we're shutting down
			select {
			case <-c.ctx.Done():
				return
			default:
				zap.L().Error("Error reading message", zap.Error(err))
				return
			}
		}

		if err := c.handleMessage(message); err != nil {
			zap.L().Error("Error handling message", zap.Error(err))
		}
	}
}

func (c *WebSocketClient) handleMessage(message []byte) error {
	var baseMsg map[string]interface{}
	if err := json.Unmarshal(message, &baseMsg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Check if this is a subscription confirmation (has "type" at root level)
	if msgType, ok := baseMsg["type"].(string); ok {
		if msgType == "subscriptions" {
			zap.L().Info("Subscription confirmed")
			return nil
		}
		if msgType == "error" {
			errMsg, _ := baseMsg["message"].(string)
			return fmt.Errorf("WebSocket error: %s", errMsg)
		}
	}

	// Check for channel (l2_data messages)
	channel, ok := baseMsg["channel"].(string)
	if !ok || channel != "l2_data" {
		return nil
	}

	// Get events array
	eventsRaw, ok := baseMsg["events"].([]interface{})
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

func (c *WebSocketClient) handleL2Event(event map[string]interface{}) error {
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
func (c *WebSocketClient) parseEventMetadata(event map[string]interface{}) (string, string, error) {
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
func (c *WebSocketClient) parseUpdates(event map[string]interface{}) (map[string]PriceLevel, error) {
	updatesRaw, ok := event["updates"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("event missing updates array")
	}

	levels := make(map[string]PriceLevel)

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

		priceLevel := PriceLevel{
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
func (c *WebSocketClient) parseDecimalField(update map[string]interface{}, key string) (decimal.Decimal, error) {
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
func (c *WebSocketClient) handleSnapshot(book *OrderBook, levels map[string]PriceLevel) error {
	bids, asks := c.buildOrderBook(levels)
	book.Update(bids, asks, 0)
	return nil
}

// handleUpdate applies incremental updates to existing order book
func (c *WebSocketClient) handleUpdate(book *OrderBook, newLevels map[string]PriceLevel) error {
	snapshot := book.Snapshot()

	// Build maps of existing levels
	bidMap := make(map[string]PriceLevel)
	askMap := make(map[string]PriceLevel)

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
func (c *WebSocketClient) buildOrderBook(levels map[string]PriceLevel) ([]PriceLevel, []PriceLevel) {
	bids := []PriceLevel{}
	asks := []PriceLevel{}

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
func (c *WebSocketClient) mapToSlice(m map[string]PriceLevel) []PriceLevel {
	result := make([]PriceLevel, 0, len(m))
	for _, level := range m {
		result = append(result, level)
	}
	return result
}

// limitLevels caps the number of levels if MaxLevels is configured
func (c *WebSocketClient) limitLevels(levels []PriceLevel) []PriceLevel {
	if c.config.MaxLevels > 0 && len(levels) > c.config.MaxLevels {
		return levels[:c.config.MaxLevels]
	}
	return levels
}

// sortBids sorts bids in descending order (highest price first)
func (c *WebSocketClient) sortBids(bids []PriceLevel) {
	sort.Slice(bids, func(i, j int) bool {
		return bids[i].Price.GreaterThan(bids[j].Price)
	})
}

// sortAsks sorts asks in ascending order (lowest price first)
func (c *WebSocketClient) sortAsks(asks []PriceLevel) {
	sort.Slice(asks, func(i, j int) bool {
		return asks[i].Price.LessThan(asks[j].Price)
	})
}
