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
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// WebSocketConfig holds configuration for the Prime Orders WebSocket connection
type WebSocketConfig struct {
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

// WebSocketClient manages the connection to Coinbase Prime Orders WebSocket
type WebSocketClient struct {
	config    WebSocketConfig
	handler   OrderUpdateHandler
	conn      *websocket.Conn
	ctx       context.Context
	cancel    context.CancelFunc
	reconnect bool
}

// NewWebSocketClient creates a new Orders WebSocket client
func NewWebSocketClient(config WebSocketConfig, handler OrderUpdateHandler) *WebSocketClient {
	ctx, cancel := context.WithCancel(context.Background())

	return &WebSocketClient{
		config:    config,
		handler:   handler,
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
			zap.L().Error("Failed to connect to orders websocket", zap.Error(err))
			time.Sleep(c.config.ReconnectDelay)
			continue
		}

		if err := c.subscribe(); err != nil {
			zap.L().Error("Failed to subscribe to orders channel", zap.Error(err))
			c.conn.Close()
			time.Sleep(c.config.ReconnectDelay)
			continue
		}

		c.readMessages()

		// Connection closed, reconnect if enabled
		if c.reconnect {
			zap.L().Info("Reconnecting to orders websocket", zap.Duration("delay", c.config.ReconnectDelay))
			time.Sleep(c.config.ReconnectDelay)
		}
	}
}

func (c *WebSocketClient) connect() error {
	zap.L().Info("Connecting to Prime Orders WebSocket", zap.String("url", c.config.Url))

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(c.config.Url, nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	c.conn = conn
	zap.L().Info("Connected to Prime Orders WebSocket")
	return nil
}

func (c *WebSocketClient) subscribe() error {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	channel := "orders"

	// Concatenate all product IDs for signature (e.g., "BTC-USDETH-USD")
	productIdsJoined := ""
	for _, p := range c.config.Products {
		productIdsJoined += p
	}

	// Create signature for WebSocket authentication
	// Format: channel + accessKey + serviceAccountId + timestamp + portfolioId + joinedProductIDs
	message := channel + c.config.AccessKey + c.config.ServiceAccountId + timestamp + c.config.PortfolioId + productIdsJoined
	signature := c.sign(message)

	// Build subscription message
	sub := map[string]interface{}{
		"type":         "subscribe",
		"channel":      channel,
		"access_key":   c.config.AccessKey,
		"api_key_id":   c.config.ServiceAccountId,
		"timestamp":    timestamp,
		"passphrase":   c.config.Passphrase,
		"signature":    signature,
		"portfolio_id": c.config.PortfolioId,
		"product_ids":  c.config.Products,
	}

	if err := c.conn.WriteJSON(sub); err != nil {
		return fmt.Errorf("failed to send subscription: %w", err)
	}

	zap.L().Info("Sent orders subscription request",
		zap.Strings("products", c.config.Products),
		zap.String("portfolio_id", c.config.PortfolioId))
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
				zap.L().Error("Error reading message from orders websocket", zap.Error(err))
				return
			}
		}

		if err := c.handleMessage(message); err != nil {
			zap.L().Error("Error handling orders message", zap.Error(err))
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
			zap.L().Info("Orders subscription confirmed")
			return nil
		}
		if msgType == "error" {
			errMsg, _ := baseMsg["message"].(string)
			return fmt.Errorf("WebSocket error: %s", errMsg)
		}
	}

	// Check for channel (orders messages)
	channel, ok := baseMsg["channel"].(string)
	if !ok || channel != "orders" {
		return nil
	}

	// Pass to handler without logging raw JSON
	if c.handler != nil {
		if err := c.handler.HandleOrderUpdate(baseMsg); err != nil {
			zap.L().Error("Handler error", zap.Error(err))
		}
	}

	return nil
}
