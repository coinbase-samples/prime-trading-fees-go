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
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// PriceLevel represents a single price level in the order book
type PriceLevel struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

// OrderBook maintains the current state of bids and asks for a product
type OrderBook struct {
	mu         sync.RWMutex
	Product    string
	Bids       []PriceLevel // Sorted descending by price
	Asks       []PriceLevel // Sorted ascending by price
	UpdateTime time.Time
	Sequence   uint64
}

// NewOrderBook creates a new order book for a product
func NewOrderBook(product string) *OrderBook {
	return &OrderBook{
		Product:    product,
		Bids:       make([]PriceLevel, 0),
		Asks:       make([]PriceLevel, 0),
		UpdateTime: time.Now(),
	}
}

// Update replaces the order book with new levels
func (ob *OrderBook) Update(bids, asks []PriceLevel, sequence uint64) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.Bids = bids
	ob.Asks = asks
	ob.Sequence = sequence
	ob.UpdateTime = time.Now()
}

// GetBestBid returns the highest bid price and size
func (ob *OrderBook) GetBestBid() (PriceLevel, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if len(ob.Bids) > 0 {
		return ob.Bids[0], true
	}
	return PriceLevel{}, false
}

// GetBestAsk returns the lowest ask price and size
func (ob *OrderBook) GetBestAsk() (PriceLevel, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if len(ob.Asks) > 0 {
		return ob.Asks[0], true
	}
	return PriceLevel{}, false
}

// GetTopLevels returns the top N levels of bids and asks
func (ob *OrderBook) GetTopLevels(n int) (bids []PriceLevel, asks []PriceLevel) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bidCount := n
	if len(ob.Bids) < bidCount {
		bidCount = len(ob.Bids)
	}

	askCount := n
	if len(ob.Asks) < askCount {
		askCount = len(ob.Asks)
	}

	bids = make([]PriceLevel, bidCount)
	copy(bids, ob.Bids[:bidCount])

	asks = make([]PriceLevel, askCount)
	copy(asks, ob.Asks[:askCount])

	return bids, asks
}

// Snapshot returns a copy of the current order book state
func (ob *OrderBook) Snapshot() OrderBookSnapshot {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bids := make([]PriceLevel, len(ob.Bids))
	copy(bids, ob.Bids)

	asks := make([]PriceLevel, len(ob.Asks))
	copy(asks, ob.Asks)

	return OrderBookSnapshot{
		Product:    ob.Product,
		Bids:       bids,
		Asks:       asks,
		UpdateTime: ob.UpdateTime,
		Sequence:   ob.Sequence,
	}
}

// OrderBookSnapshot is an immutable snapshot of the order book
type OrderBookSnapshot struct {
	Product    string
	Bids       []PriceLevel
	Asks       []PriceLevel
	UpdateTime time.Time
	Sequence   uint64
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
