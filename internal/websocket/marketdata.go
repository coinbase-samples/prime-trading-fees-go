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
	"sync"
	"sync/atomic"
	"time"

	"github.com/coinbase-samples/prime-trading-fees-go/internal/common"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// ============================================================================
// Order Book Data Structures
// ============================================================================

// OrderBook maintains the current state of bids and asks for a product
type OrderBook struct {
	mu           sync.Mutex // only writers use this
	Product      string
	Bids         []common.PriceLevel // Sorted descending by price
	Asks         []common.PriceLevel // Sorted ascending by price
	UpdateTime   time.Time
	Sequence     uint64
	bestBidValue atomic.Value // stores common.PriceLevel or nil
	bestAskValue atomic.Value // stores common.PriceLevel or nil
}

// NewOrderBook creates a new order book for a product
func NewOrderBook(product string) *OrderBook {
	return &OrderBook{
		Product:    product,
		Bids:       make([]common.PriceLevel, 0),
		Asks:       make([]common.PriceLevel, 0),
		UpdateTime: time.Now(),
	}
}

// Update replaces the order book with new levels
func (ob *OrderBook) Update(bids, asks []common.PriceLevel, sequence uint64) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.Bids = bids
	ob.Asks = asks
	ob.Sequence = sequence
	ob.UpdateTime = time.Now()

	// Update atomic best bid/ask
	if len(bids) > 0 {
		ob.bestBidValue.Store(bids[0])
	} else {
		ob.bestBidValue.Store(nil)
	}

	if len(asks) > 0 {
		ob.bestAskValue.Store(asks[0])
	} else {
		ob.bestAskValue.Store(nil)
	}
}

// GetBestBid returns the highest bid price and size
func (ob *OrderBook) GetBestBid() (common.PriceLevel, bool) {
	v := ob.bestBidValue.Load()
	if v == nil {
		return common.PriceLevel{}, false
	}
	return v.(common.PriceLevel), true
}

// GetBestAsk returns the lowest ask price and size
func (ob *OrderBook) GetBestAsk() (common.PriceLevel, bool) {
	v := ob.bestAskValue.Load()
	if v == nil {
		return common.PriceLevel{}, false
	}
	return v.(common.PriceLevel), true
}

// GetTopLevels returns the top N levels of bids and asks
func (ob *OrderBook) GetTopLevels(n int) (bids []common.PriceLevel, asks []common.PriceLevel) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	bidCount := n
	if len(ob.Bids) < bidCount {
		bidCount = len(ob.Bids)
	}

	askCount := n
	if len(ob.Asks) < askCount {
		askCount = len(ob.Asks)
	}

	bids = make([]common.PriceLevel, bidCount)
	copy(bids, ob.Bids[:bidCount])

	asks = make([]common.PriceLevel, askCount)
	copy(asks, ob.Asks[:askCount])

	return bids, asks
}

// Snapshot returns a copy of the current order book state
func (ob *OrderBook) Snapshot() common.OrderBookSnapshot {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	bids := make([]common.PriceLevel, len(ob.Bids))
	copy(bids, ob.Bids)

	asks := make([]common.PriceLevel, len(ob.Asks))
	copy(asks, ob.Asks)

	return common.OrderBookSnapshot{
		Product:    ob.Product,
		Bids:       bids,
		Asks:       asks,
		UpdateTime: ob.UpdateTime,
		Sequence:   ob.Sequence,
	}
}

// OrderBookStore manages multiple order books
type OrderBookStore struct {
	mu    sync.RWMutex
	books map[string]*OrderBook
}

// NewOrderBookStore creates a new order book store
func NewOrderBookStore() *OrderBookStore {
	return &OrderBookStore{
		books: make(map[string]*OrderBook),
	}
}

// GetOrCreate retrieves or creates an order book for a product
func (s *OrderBookStore) GetOrCreate(product string) *OrderBook {
	s.mu.Lock()
	defer s.mu.Unlock()

	if book, exists := s.books[product]; exists {
		return book
	}

	book := NewOrderBook(product)
	s.books[product] = book
	return book
}

// Get retrieves an order book for a product
func (s *OrderBookStore) Get(product string) (*OrderBook, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	book, exists := s.books[product]
	return book, exists
}

// ============================================================================
// Market Data WebSocket Client
// ============================================================================

// MarketDataConfig holds configuration for the Prime WebSocket connection
type MarketDataConfig struct {
	CommonConfig
	Portfolio string
	MaxLevels int
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

	baseConfig := baseConfigFromCommon(config.CommonConfig)
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
	// Format: channel + accessKey + serviceAccountId + timestamp + joinedProductIDs
	productIdsJoined := joinProductIds(c.config.Products)
	return c.GetChannelName() + baseConfig.AccessKey + baseConfig.ServiceAccountId + timestamp + productIdsJoined
}

// BuildSubscriptionMessage builds the subscription payload
func (c *MarketDataClient) BuildSubscriptionMessage(baseConfig BaseConfig, timestamp string, signature string) map[string]interface{} {
	return buildBaseSubscriptionMessage(
		c.GetChannelName(),
		baseConfig.AccessKey,
		baseConfig.ServiceAccountId,
		timestamp,
		baseConfig.Passphrase,
		signature,
		c.config.Products,
	)
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
