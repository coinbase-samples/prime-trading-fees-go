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
	"time"

	"go.uber.org/zap"
)

// OrdersConfig holds configuration for the Prime Orders WebSocket connection
type OrdersConfig struct {
	Url              string
	AccessKey        string
	Passphrase       string
	SigningKey       string
	ServiceAccountId string
	PortfolioId      string
	Products         []string
	ReconnectDelay   time.Duration
}

// OrderUpdateHandler processes order updates from the websocket
type OrderUpdateHandler interface {
	HandleOrderUpdate(update map[string]interface{}) error
}

// OrdersClient manages the connection to Coinbase Prime Orders WebSocket
type OrdersClient struct {
	config     OrdersConfig
	handler    OrderUpdateHandler
	baseClient *BaseWebSocketClient
}

// NewOrdersClient creates a new Orders WebSocket client
func NewOrdersClient(config OrdersConfig, handler OrderUpdateHandler) *OrdersClient {
	client := &OrdersClient{
		config:  config,
		handler: handler,
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
func (c *OrdersClient) Start() error {
	return c.baseClient.Start()
}

// Stop gracefully stops the WebSocket client
func (c *OrdersClient) Stop() {
	c.baseClient.Stop()
}

// ChannelHandler interface implementation

// GetChannelName returns the channel name for this handler
func (c *OrdersClient) GetChannelName() string {
	return "orders"
}

// BuildSignatureMessage builds the message string to be signed
func (c *OrdersClient) BuildSignatureMessage(baseConfig BaseConfig, timestamp string) string {
	// Concatenate all product IDs for signature (e.g., "BTC-USDETH-USD")
	productIdsJoined := ""
	for _, p := range c.config.Products {
		productIdsJoined += p
	}

	// Format: channel + accessKey + serviceAccountId + timestamp + portfolioId + joinedProductIDs
	return c.GetChannelName() + baseConfig.AccessKey + baseConfig.ServiceAccountId + timestamp + c.config.PortfolioId + productIdsJoined
}

// BuildSubscriptionMessage builds the subscription payload
func (c *OrdersClient) BuildSubscriptionMessage(baseConfig BaseConfig, timestamp string, signature string) map[string]interface{} {
	return map[string]interface{}{
		"type":         "subscribe",
		"channel":      c.GetChannelName(),
		"access_key":   baseConfig.AccessKey,
		"api_key_id":   baseConfig.ServiceAccountId,
		"timestamp":    timestamp,
		"passphrase":   baseConfig.Passphrase,
		"signature":    signature,
		"portfolio_id": c.config.PortfolioId,
		"product_ids":  c.config.Products,
	}
}

// HandleMessage processes messages for the orders channel
func (c *OrdersClient) HandleMessage(message map[string]interface{}) error {
	// Pass to handler without logging raw JSON
	if c.handler != nil {
		if err := c.handler.HandleOrderUpdate(message); err != nil {
			zap.L().Error("Handler error", zap.Error(err))
		}
	}

	return nil
}
