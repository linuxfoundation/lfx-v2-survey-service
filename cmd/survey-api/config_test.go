// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	apieventing "github.com/linuxfoundation/lfx-v2-survey-service/cmd/survey-api/eventing"
	"github.com/stretchr/testify/assert"
)

// configEnvVars lists every variable loadConfig reads, so tests start from a clean slate.
var configEnvVars = []string{
	"PORT", "JWKS_URL", "AUDIENCE", "JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL",
	"ITX_BASE_URL", "ITX_AUTH0_DOMAIN", "ITX_CLIENT_ID", "ITX_CLIENT_PRIVATE_KEY", "ITX_AUDIENCE",
	"NATS_URL", "ID_MAPPING_DISABLED", "EVENT_PROCESSING_ENABLED", "EVENT_CONSUMER_NAME",
	"EVENT_STREAM_NAME", "INVITES_ENABLED", "LFX_SELF_SERVE_BASE_URL", "LFX_ENVIRONMENT",
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range configEnvVars {
		t.Setenv(k, "")
	}
}

func TestGetEnv(t *testing.T) {
	t.Run("returns the variable when set", func(t *testing.T) {
		t.Setenv("SURVEY_TEST_GETENV", "from-env")
		assert.Equal(t, "from-env", getEnv("SURVEY_TEST_GETENV", "fallback"))
	})

	t.Run("returns the default when unset", func(t *testing.T) {
		assert.Equal(t, "fallback", getEnv("SURVEY_TEST_GETENV_UNSET", "fallback"))
	})

	t.Run("returns the default when set to the empty string", func(t *testing.T) {
		t.Setenv("SURVEY_TEST_GETENV", "")
		assert.Equal(t, "fallback", getEnv("SURVEY_TEST_GETENV", "fallback"))
	})
}

func TestLoadConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		clearConfigEnv(t)

		cfg := loadConfig()

		assert.Equal(t, "8080", cfg.Port)
		assert.Equal(t, "http://heimdall:4457/.well-known/jwks", cfg.JWKSURL)
		assert.Equal(t, "lfx-v2-survey-service", cfg.Audience)
		assert.Empty(t, cfg.MockLocalPrincipal)
		assert.Equal(t, "https://api.dev.itx.linuxfoundation.org/", cfg.ITXBaseURL)
		assert.Equal(t, "linuxfoundation-dev.auth0.com", cfg.ITXAuth0Domain)
		assert.Empty(t, cfg.ITXClientID)
		assert.Empty(t, cfg.ITXPrivateKey)
		assert.Equal(t, "https://api.dev.itx.linuxfoundation.org/", cfg.ITXAudience)
		assert.Equal(t, 30*time.Second, cfg.ITXTimeout)
		assert.Equal(t, "nats://nats:4222", cfg.NATSURL)
		assert.Equal(t, 5*time.Second, cfg.NATSTimeout)
		assert.False(t, cfg.IDMappingDisabled)
		assert.True(t, cfg.EventProcessingEnabled, "event processing is on unless disabled")
		assert.Equal(t, "survey-service-kv-consumer", cfg.EventConsumerName)
		assert.Equal(t, "KV_v1-objects", cfg.EventStreamName)
		assert.False(t, cfg.InvitesEnabled)
		assert.Empty(t, cfg.SelfServeBaseURL)
		assert.Equal(t, "dev", cfg.LFXEnvironment)
	})

	t.Run("every variable overrides its default", func(t *testing.T) {
		clearConfigEnv(t)
		t.Setenv("PORT", "9090")
		t.Setenv("JWKS_URL", "http://jwks.example.test/keys")
		t.Setenv("AUDIENCE", "audience-override")
		t.Setenv("JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL", "local-user")
		t.Setenv("ITX_BASE_URL", "https://itx.example.test/")
		t.Setenv("ITX_AUTH0_DOMAIN", "auth0.example.test")
		t.Setenv("ITX_CLIENT_ID", "client-id-1")
		t.Setenv("ITX_CLIENT_PRIVATE_KEY", "private-key-1")
		t.Setenv("ITX_AUDIENCE", "https://itx-audience.example.test/")
		t.Setenv("NATS_URL", "nats://nats.example.test:4222")
		t.Setenv("ID_MAPPING_DISABLED", "true")
		t.Setenv("EVENT_PROCESSING_ENABLED", "false")
		t.Setenv("EVENT_CONSUMER_NAME", "consumer-override")
		t.Setenv("EVENT_STREAM_NAME", "stream-override")
		t.Setenv("INVITES_ENABLED", "true")
		t.Setenv("LFX_SELF_SERVE_BASE_URL", "https://self-serve.example.test")
		t.Setenv("LFX_ENVIRONMENT", "staging")

		cfg := loadConfig()

		assert.Equal(t, "9090", cfg.Port)
		assert.Equal(t, "http://jwks.example.test/keys", cfg.JWKSURL)
		assert.Equal(t, "audience-override", cfg.Audience)
		assert.Equal(t, "local-user", cfg.MockLocalPrincipal)
		assert.Equal(t, "https://itx.example.test/", cfg.ITXBaseURL)
		assert.Equal(t, "auth0.example.test", cfg.ITXAuth0Domain)
		assert.Equal(t, "client-id-1", cfg.ITXClientID)
		assert.Equal(t, "private-key-1", cfg.ITXPrivateKey)
		assert.Equal(t, "https://itx-audience.example.test/", cfg.ITXAudience)
		assert.Equal(t, "nats://nats.example.test:4222", cfg.NATSURL)
		assert.True(t, cfg.IDMappingDisabled)
		assert.False(t, cfg.EventProcessingEnabled)
		assert.Equal(t, "consumer-override", cfg.EventConsumerName)
		assert.Equal(t, "stream-override", cfg.EventStreamName)
		assert.True(t, cfg.InvitesEnabled)
		assert.Equal(t, "https://self-serve.example.test", cfg.SelfServeBaseURL)
		assert.Equal(t, "staging", cfg.LFXEnvironment)
	})

	t.Run("boolean flags only accept the exact string true", func(t *testing.T) {
		for _, value := range []string{"1", "TRUE", "True", "yes", "false"} {
			clearConfigEnv(t)
			t.Setenv("ID_MAPPING_DISABLED", value)
			t.Setenv("INVITES_ENABLED", value)
			t.Setenv("EVENT_PROCESSING_ENABLED", value)

			cfg := loadConfig()

			assert.False(t, cfg.IDMappingDisabled, "ID_MAPPING_DISABLED=%q", value)
			assert.False(t, cfg.InvitesEnabled, "INVITES_ENABLED=%q", value)
			assert.False(t, cfg.EventProcessingEnabled, "EVENT_PROCESSING_ENABLED=%q", value)
		}
	})
}

func TestConfigValidate(t *testing.T) {
	valid := config{ITXClientID: "client-id-1", ITXPrivateKey: "private-key-1"}

	t.Run("accepts a config with both ITX credentials", func(t *testing.T) {
		assert.NoError(t, valid.validate())
	})

	t.Run("requires ITX_CLIENT_ID", func(t *testing.T) {
		cfg := valid
		cfg.ITXClientID = ""
		err := cfg.validate()
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "ITX_CLIENT_ID")
		}
	})

	t.Run("requires ITX_CLIENT_PRIVATE_KEY", func(t *testing.T) {
		cfg := valid
		cfg.ITXPrivateKey = ""
		err := cfg.validate()
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "ITX_CLIENT_PRIVATE_KEY")
		}
	})
}

func TestSelfServeBaseURLForEnv(t *testing.T) {
	tests := []struct {
		env  string
		want string
	}{
		{env: "prod", want: "https://lfx.linuxfoundation.org"},
		{env: "production", want: "https://lfx.linuxfoundation.org"},
		{env: "staging", want: "https://lfx.staging.platform.linuxfoundation.org"},
		{env: "stage", want: "https://lfx.staging.platform.linuxfoundation.org"},
		{env: "dev", want: "https://lfx.dev.platform.linuxfoundation.org"},
		{env: "local", want: "https://lfx.dev.platform.linuxfoundation.org"},
		{env: "", want: "https://lfx.dev.platform.linuxfoundation.org"},
		{env: "unknown", want: "https://lfx.dev.platform.linuxfoundation.org"},
	}
	for _, tt := range tests {
		t.Run("env="+tt.env, func(t *testing.T) {
			assert.Equal(t, tt.want, selfServeBaseURLForEnv(tt.env))
		})
	}
}

func TestParseInviteConfig(t *testing.T) {
	newLogger := func() (*slog.Logger, *bytes.Buffer) {
		var buf bytes.Buffer
		return slog.New(slog.NewTextHandler(&buf, nil)), &buf
	}

	t.Run("disabled returns the zero config even when a base URL is set", func(t *testing.T) {
		logger, buf := newLogger()

		got := parseInviteConfig(config{InvitesEnabled: false, SelfServeBaseURL: "https://self-serve.example.test"}, logger)

		assert.Equal(t, apieventing.InviteFeatureConfig{}, got)
		assert.Contains(t, buf.String(), "DISABLED")
	})

	t.Run("enabled uses the explicit base URL without logging a fallback", func(t *testing.T) {
		logger, buf := newLogger()

		got := parseInviteConfig(config{
			InvitesEnabled:   true,
			SelfServeBaseURL: "https://self-serve.example.test",
			LFXEnvironment:   "prod",
		}, logger)

		assert.Equal(t, apieventing.InviteFeatureConfig{Enabled: true, SelfServeBaseURL: "https://self-serve.example.test"}, got)
		assert.False(t, strings.Contains(buf.String(), "not set"), "no fallback log expected: %s", buf.String())
	})

	t.Run("enabled without a base URL falls back to the environment default", func(t *testing.T) {
		logger, buf := newLogger()

		got := parseInviteConfig(config{InvitesEnabled: true, LFXEnvironment: "prod"}, logger)

		assert.Equal(t, apieventing.InviteFeatureConfig{Enabled: true, SelfServeBaseURL: "https://lfx.linuxfoundation.org"}, got)
		assert.Contains(t, buf.String(), "LFX_SELF_SERVE_BASE_URL not set")
		assert.Contains(t, buf.String(), "https://lfx.linuxfoundation.org")
	})
}
