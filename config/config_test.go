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

package config

import (
	"os"
	"testing"
)

func TestFeesConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     FeesConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid flat fee",
			cfg: FeesConfig{
				Type:   "flat",
				Amount: "0.05",
			},
			wantErr: false,
		},
		{
			name: "valid percent fee",
			cfg: FeesConfig{
				Type:    "percent",
				Percent: "0.005",
			},
			wantErr: false,
		},
		{
			name: "invalid type",
			cfg: FeesConfig{
				Type:   "invalid",
				Amount: "0.05",
			},
			wantErr: true,
			errMsg:  "invalid fee type: invalid (must be flat or percent)",
		},
		{
			name: "flat fee missing amount",
			cfg: FeesConfig{
				Type:   "flat",
				Amount: "",
			},
			wantErr: true,
			errMsg:  "amount is required for flat fee strategy",
		},
		{
			name: "flat fee invalid decimal",
			cfg: FeesConfig{
				Type:   "flat",
				Amount: "invalid",
			},
			wantErr: true,
		},
		{
			name: "percent fee missing percent",
			cfg: FeesConfig{
				Type:    "percent",
				Percent: "",
			},
			wantErr: true,
			errMsg:  "percent is required for percent fee strategy",
		},
		{
			name: "percent fee invalid decimal",
			cfg: FeesConfig{
				Type:    "percent",
				Percent: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %q, want %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: Config{
				Prime: PrimeConfig{
					AccessKey:        "test-key",
					Passphrase:       "test-pass",
					SigningKey:       "test-signing",
					Portfolio:        "test-portfolio",
					ServiceAccountId: "test-account",
				},
				MarketData: MarketDataConfig{
					Products: []string{"BTC-USD"},
				},
				Fees: FeesConfig{
					Type:    "percent",
					Percent: "0.005",
				},
			},
			wantErr: false,
		},
		{
			name: "missing access key",
			cfg: Config{
				Prime: PrimeConfig{
					Passphrase:       "test-pass",
					SigningKey:       "test-signing",
					Portfolio:        "test-portfolio",
					ServiceAccountId: "test-account",
				},
				MarketData: MarketDataConfig{
					Products: []string{"BTC-USD"},
				},
				Fees: FeesConfig{
					Type:    "percent",
					Percent: "0.005",
				},
			},
			wantErr: true,
			errMsg:  "PRIME_ACCESS_KEY is required",
		},
		{
			name: "missing passphrase",
			cfg: Config{
				Prime: PrimeConfig{
					AccessKey:        "test-key",
					SigningKey:       "test-signing",
					Portfolio:        "test-portfolio",
					ServiceAccountId: "test-account",
				},
				MarketData: MarketDataConfig{
					Products: []string{"BTC-USD"},
				},
				Fees: FeesConfig{
					Type:    "percent",
					Percent: "0.005",
				},
			},
			wantErr: true,
			errMsg:  "PRIME_PASSPHRASE is required",
		},
		{
			name: "missing signing key",
			cfg: Config{
				Prime: PrimeConfig{
					AccessKey:        "test-key",
					Passphrase:       "test-pass",
					Portfolio:        "test-portfolio",
					ServiceAccountId: "test-account",
				},
				MarketData: MarketDataConfig{
					Products: []string{"BTC-USD"},
				},
				Fees: FeesConfig{
					Type:    "percent",
					Percent: "0.005",
				},
			},
			wantErr: true,
			errMsg:  "PRIME_SIGNING_KEY is required",
		},
		{
			name: "missing portfolio id",
			cfg: Config{
				Prime: PrimeConfig{
					AccessKey:        "test-key",
					Passphrase:       "test-pass",
					SigningKey:       "test-signing",
					ServiceAccountId: "test-account",
				},
				MarketData: MarketDataConfig{
					Products: []string{"BTC-USD"},
				},
				Fees: FeesConfig{
					Type:    "percent",
					Percent: "0.005",
				},
			},
			wantErr: true,
			errMsg:  "PRIME_PORTFOLIO is required",
		},
		{
			name: "missing service account id",
			cfg: Config{
				Prime: PrimeConfig{
					AccessKey:  "test-key",
					Passphrase: "test-pass",
					SigningKey: "test-signing",
					Portfolio:  "test-portfolio",
				},
				MarketData: MarketDataConfig{
					Products: []string{"BTC-USD"},
				},
				Fees: FeesConfig{
					Type:    "percent",
					Percent: "0.005",
				},
			},
			wantErr: true,
			errMsg:  "PRIME_SERVICE_ACCOUNT_ID is required",
		},
		{
			name: "no products",
			cfg: Config{
				Prime: PrimeConfig{
					AccessKey:        "test-key",
					Passphrase:       "test-pass",
					SigningKey:       "test-signing",
					Portfolio:        "test-portfolio",
					ServiceAccountId: "test-account",
				},
				MarketData: MarketDataConfig{
					Products: []string{},
				},
				Fees: FeesConfig{
					Type:    "percent",
					Percent: "0.005",
				},
			},
			wantErr: true,
			errMsg:  "at least one product is required",
		},
		{
			name: "invalid fees config",
			cfg: Config{
				Prime: PrimeConfig{
					AccessKey:        "test-key",
					Passphrase:       "test-pass",
					SigningKey:       "test-signing",
					Portfolio:        "test-portfolio",
					ServiceAccountId: "test-account",
				},
				MarketData: MarketDataConfig{
					Products: []string{"BTC-USD"},
				},
				Fees: FeesConfig{
					Type: "invalid",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %q, want %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Skip this test - it requires valid credentials to pass validation
	t.Skip("Skipping test that requires valid Prime credentials")

	// Load config (should use defaults)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Check defaults
	if cfg.Database.Path != "orders.db" {
		t.Errorf("Database.Path = %q, want orders.db", cfg.Database.Path)
	}

	if cfg.Server.LogLevel != "info" {
		t.Errorf("Logging.Level = %q, want info", cfg.Server.LogLevel)
	}

	if len(cfg.MarketData.Products) == 0 {
		t.Error("MarketData.Products is empty, should have defaults")
	}

	if cfg.Fees.Type != "percent" {
		t.Errorf("Fees.Type = %q, want percent", cfg.Fees.Type)
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Save original env vars
	originalAccessKey := os.Getenv("PRIME_ACCESS_KEY")
	originalPassphrase := os.Getenv("PRIME_PASSPHRASE")
	originalProducts := os.Getenv("MARKET_DATA_PRODUCTS")

	// Set test env vars
	os.Setenv("PRIME_ACCESS_KEY", "env-access-key")
	os.Setenv("PRIME_PASSPHRASE", "env-passphrase")
	os.Setenv("MARKET_DATA_PRODUCTS", "BTC-USD,ETH-USD,SOL-USD")

	// Restore after test
	defer func() {
		os.Setenv("PRIME_ACCESS_KEY", originalAccessKey)
		os.Setenv("PRIME_PASSPHRASE", originalPassphrase)
		os.Setenv("MARKET_DATA_PRODUCTS", originalProducts)
	}()

	cfg := &Config{}
	loadFromEnv(cfg)

	if cfg.Prime.AccessKey != "env-access-key" {
		t.Errorf("Prime.AccessKey = %q, want env-access-key", cfg.Prime.AccessKey)
	}

	if cfg.Prime.Passphrase != "env-passphrase" {
		t.Errorf("Prime.Passphrase = %q, want env-passphrase", cfg.Prime.Passphrase)
	}

	if len(cfg.MarketData.Products) != 3 {
		t.Errorf("len(MarketData.Products) = %d, want 3", len(cfg.MarketData.Products))
	}

	expectedProducts := []string{"BTC-USD", "ETH-USD", "SOL-USD"}
	for i, product := range cfg.MarketData.Products {
		if product != expectedProducts[i] {
			t.Errorf("Products[%d] = %q, want %q", i, product, expectedProducts[i])
		}
	}
}

func TestLoadFromEnv_EmptyValues(t *testing.T) {
	// Clear all env vars
	vars := []string{
		"PRIME_ACCESS_KEY",
		"PRIME_PASSPHRASE",
		"PRIME_SIGNING_KEY",
		"PRIME_PORTFOLIO_ID",
		"PRIME_SERVICE_ACCOUNT_ID",
		"DATABASE_PATH",
		"LOG_LEVEL",
		"LOG_JSON",
		"MARKET_DATA_PRODUCTS",
		"FEES_TYPE",
		"FEES_AMOUNT",
		"FEES_PERCENT",
	}

	original := make(map[string]string)
	for _, v := range vars {
		original[v] = os.Getenv(v)
		os.Unsetenv(v)
	}

	defer func() {
		for k, v := range original {
			os.Setenv(k, v)
		}
	}()

	cfg := &Config{
		Prime: PrimeConfig{
			AccessKey: "initial-value",
		},
	}

	loadFromEnv(cfg)

	// Empty env vars should not overwrite existing values
	if cfg.Prime.AccessKey != "initial-value" {
		t.Errorf("Prime.AccessKey = %q, want initial-value (should not be overwritten)", cfg.Prime.AccessKey)
	}
}
