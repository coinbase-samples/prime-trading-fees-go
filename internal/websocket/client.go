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

// BaseConfig holds common configuration for Prime WebSocket connections
type BaseConfig struct {
	Url              string
	AccessKey        string
	Passphrase       string
	SigningKey       string
	ServiceAccountId string
	ReconnectDelay   time.Duration
}

// ChannelHandler processes messages for a specific channel
type ChannelHandler interface {
	// HandleMessage processes a message for this channel
	HandleMessage(message map[string]interface{}) error

	// GetChannelName returns the channel name (e.g., "l2_data", "orders")
	GetChannelName() string

	// BuildSubscriptionMessage builds the subscription payload for this channel
	// Returns the subscription object to be sent as JSON
	BuildSubscriptionMessage(baseConfig BaseConfig, timestamp string, signature string) map[string]interface{}

	// BuildSignatureMessage builds the message string to be signed
	// Returns the message that will be signed with HMAC-SHA256
	BuildSignatureMessage(baseConfig BaseConfig, timestamp string) string
}

// BaseWebSocketClient manages a WebSocket connection with common functionality
type BaseWebSocketClient struct {
	config    BaseConfig
	handler   ChannelHandler
	conn      *websocket.Conn
	ctx       context.Context
	cancel    context.CancelFunc
	reconnect bool
}

// NewBaseWebSocketClient creates a new base WebSocket client
func NewBaseWebSocketClient(config BaseConfig, handler ChannelHandler) *BaseWebSocketClient {
	ctx, cancel := context.WithCancel(context.Background())

	return &BaseWebSocketClient{
		config:    config,
		handler:   handler,
		ctx:       ctx,
		cancel:    cancel,
		reconnect: true,
	}
}

// Start begins the WebSocket connection and message processing
func (c *BaseWebSocketClient) Start() error {
	go c.run()
	return nil
}

// Stop gracefully stops the WebSocket client
func (c *BaseWebSocketClient) Stop() {
	c.reconnect = false
	c.cancel()
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *BaseWebSocketClient) run() {
	for c.reconnect {
		if err := c.connect(); err != nil {
			zap.L().Error("Failed to connect",
				zap.String("channel", c.handler.GetChannelName()),
				zap.Error(err))
			time.Sleep(c.config.ReconnectDelay)
			continue
		}

		if err := c.subscribe(); err != nil {
			zap.L().Error("Failed to subscribe",
				zap.String("channel", c.handler.GetChannelName()),
				zap.Error(err))
			c.conn.Close()
			time.Sleep(c.config.ReconnectDelay)
			continue
		}

		c.readMessages()

		// Connection closed, reconnect if enabled
		if c.reconnect {
			zap.L().Info("Reconnecting",
				zap.String("channel", c.handler.GetChannelName()),
				zap.Duration("delay", c.config.ReconnectDelay))
			time.Sleep(c.config.ReconnectDelay)
		}
	}
}

func (c *BaseWebSocketClient) connect() error {
	zap.L().Info("Connecting to Prime WebSocket",
		zap.String("url", c.config.Url),
		zap.String("channel", c.handler.GetChannelName()))

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(c.config.Url, nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	c.conn = conn
	zap.L().Info("Connected to Prime WebSocket",
		zap.String("channel", c.handler.GetChannelName()))
	return nil
}

func (c *BaseWebSocketClient) subscribe() error {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	// Build signature message using channel-specific logic
	signatureMessage := c.handler.BuildSignatureMessage(c.config, timestamp)
	signature := c.sign(signatureMessage)

	// Build subscription message using channel-specific logic
	sub := c.handler.BuildSubscriptionMessage(c.config, timestamp, signature)

	if err := c.conn.WriteJSON(sub); err != nil {
		return fmt.Errorf("failed to send subscription: %w", err)
	}

	zap.L().Info("Sent subscription request",
		zap.String("channel", c.handler.GetChannelName()))
	return nil
}

func (c *BaseWebSocketClient) sign(message string) string {
	h := hmac.New(sha256.New, []byte(c.config.SigningKey))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (c *BaseWebSocketClient) readMessages() {
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
				zap.L().Error("Error reading message",
					zap.String("channel", c.handler.GetChannelName()),
					zap.Error(err))
				return
			}
		}

		if err := c.handleMessage(message); err != nil {
			zap.L().Error("Error handling message",
				zap.String("channel", c.handler.GetChannelName()),
				zap.Error(err))
		}
	}
}

func (c *BaseWebSocketClient) handleMessage(message []byte) error {
	var baseMsg map[string]interface{}
	if err := json.Unmarshal(message, &baseMsg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Check if this is a subscription confirmation (has "type" at root level)
	if msgType, ok := baseMsg["type"].(string); ok {
		if msgType == "subscriptions" {
			zap.L().Info("Subscription confirmed",
				zap.String("channel", c.handler.GetChannelName()))
			return nil
		}
		if msgType == "error" {
			errMsg, _ := baseMsg["message"].(string)
			return fmt.Errorf("WebSocket error: %s", errMsg)
		}
	}

	// Check for channel match
	channel, ok := baseMsg["channel"].(string)
	if !ok || channel != c.handler.GetChannelName() {
		return nil
	}

	// Delegate to channel-specific handler
	return c.handler.HandleMessage(baseMsg)
}
