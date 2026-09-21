// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	apieventing "github.com/linuxfoundation/lfx-v2-survey-service/cmd/survey-api/eventing"
)

// config holds the application configuration
type config struct {
	Port                   string
	JWKSURL                string
	Audience               string
	MockLocalPrincipal     string
	ITXBaseURL             string
	ITXAuth0Domain         string
	ITXClientID            string
	ITXPrivateKey          string
	ITXAudience            string
	ITXTimeout             time.Duration
	NATSURL                string
	NATSTimeout            time.Duration
	IDMappingDisabled      bool
	EventProcessingEnabled bool
	EventConsumerName      string
	EventStreamName        string
	// Invite feature
	InvitesEnabled   bool
	SelfServeBaseURL string
	LFXEnvironment   string
}

// loadConfig loads configuration from environment variables
func loadConfig() config {
	return config{
		Port:                   getEnv("PORT", "8080"),
		JWKSURL:                getEnv("JWKS_URL", "http://heimdall:4457/.well-known/jwks"),
		Audience:               getEnv("AUDIENCE", "lfx-v2-survey-service"),
		MockLocalPrincipal:     getEnv("JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL", ""),
		ITXBaseURL:             getEnv("ITX_BASE_URL", "https://api.dev.itx.linuxfoundation.org/"),
		ITXAuth0Domain:         getEnv("ITX_AUTH0_DOMAIN", "linuxfoundation-dev.auth0.com"),
		ITXClientID:            getEnv("ITX_CLIENT_ID", ""),
		ITXPrivateKey:          getEnv("ITX_CLIENT_PRIVATE_KEY", ""),
		ITXAudience:            getEnv("ITX_AUDIENCE", "https://api.dev.itx.linuxfoundation.org/"),
		ITXTimeout:             30 * time.Second,
		NATSURL:                getEnv("NATS_URL", "nats://nats:4222"),
		NATSTimeout:            5 * time.Second,
		IDMappingDisabled:      getEnv("ID_MAPPING_DISABLED", "") == "true",
		EventProcessingEnabled: getEnv("EVENT_PROCESSING_ENABLED", "true") == "true",
		EventConsumerName:      getEnv("EVENT_CONSUMER_NAME", "survey-service-kv-consumer"),
		EventStreamName:        getEnv("EVENT_STREAM_NAME", "KV_v1-objects"),
		InvitesEnabled:         getEnv("INVITES_ENABLED", "false") == "true",
		SelfServeBaseURL:       getEnv("LFX_SELF_SERVE_BASE_URL", ""),
		LFXEnvironment:         getEnv("LFX_ENVIRONMENT", "dev"),
	}
}

// validate checks that required configuration values are set
func (c config) validate() error {
	if c.ITXClientID == "" {
		return fmt.Errorf("ITX_CLIENT_ID is required")
	}
	if c.ITXPrivateKey == "" {
		return fmt.Errorf("ITX_CLIENT_PRIVATE_KEY is required")
	}
	return nil
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// selfServeBaseURLForEnv returns the default LFX self-serve base URL for the given environment.
// The result is used when LFX_SELF_SERVE_BASE_URL is not explicitly set.
func selfServeBaseURLForEnv(env string) string {
	switch env {
	case "prod", "production":
		return "https://lfx.linuxfoundation.org"
	case "staging", "stage":
		return "https://lfx.staging.platform.linuxfoundation.org"
	default: // dev, local, or anything else
		return "https://lfx.dev.platform.linuxfoundation.org"
	}
}

// parseInviteConfig resolves the InviteFeatureConfig from config and env defaults.
func parseInviteConfig(cfg config, logger *slog.Logger) apieventing.InviteFeatureConfig {
	if !cfg.InvitesEnabled {
		logger.Info("Invite feature is DISABLED (INVITES_ENABLED != true)")
		return apieventing.InviteFeatureConfig{}
	}

	baseURL := cfg.SelfServeBaseURL
	if baseURL == "" {
		baseURL = selfServeBaseURLForEnv(cfg.LFXEnvironment)
		logger.Info("LFX_SELF_SERVE_BASE_URL not set; using environment default",
			"env", cfg.LFXEnvironment,
			"base_url", baseURL,
		)
	}

	return apieventing.InviteFeatureConfig{
		Enabled:          true,
		SelfServeBaseURL: baseURL,
	}
}
