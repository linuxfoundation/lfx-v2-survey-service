// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package utils

import (
	"context"
	"os"
	"strings"
	"testing"
)

// tUnsetenv ensures key is unset for the duration of the test and restores the
// original value (if any) via t.Cleanup.
func tUnsetenv(t *testing.T, key string) {
	t.Helper()
	old, exists := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("os.Unsetenv(%q): %v", key, err)
	}
	t.Cleanup(func() {
		if exists {
			_ = os.Setenv(key, old)
		}
	})
}

// TestOTelConfigFromEnv_Defaults verifies that OTelConfigFromEnv returns
// sensible default values when no environment variables are set.
func TestOTelConfigFromEnv_Defaults(t *testing.T) {
	tUnsetenv(t, "OTEL_SERVICE_NAME")
	tUnsetenv(t, "OTEL_SERVICE_VERSION")

	cfg := OTelConfigFromEnv()

	if cfg.ServiceName != "lfx-v2-survey-service" {
		t.Errorf("expected default ServiceName 'lfx-v2-survey-service', got %q", cfg.ServiceName)
	}
	if cfg.ServiceVersion != "" {
		t.Errorf("expected empty ServiceVersion, got %q", cfg.ServiceVersion)
	}
}

// TestOTelConfigFromEnv_CustomValues verifies that OTelConfigFromEnv correctly
// reads OTEL_SERVICE_NAME and OTEL_SERVICE_VERSION environment variables.
func TestOTelConfigFromEnv_CustomValues(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "test-service")
	t.Setenv("OTEL_SERVICE_VERSION", "1.2.3")

	cfg := OTelConfigFromEnv()

	if cfg.ServiceName != "test-service" {
		t.Errorf("expected ServiceName 'test-service', got %q", cfg.ServiceName)
	}
	if cfg.ServiceVersion != "1.2.3" {
		t.Errorf("expected ServiceVersion '1.2.3', got %q", cfg.ServiceVersion)
	}
}

// TestSetupOTelSDKWithConfig_AllDisabled verifies that the SDK can be
// initialized successfully when all exporters are set to "none", and that the
// returned shutdown function works correctly.
func TestSetupOTelSDKWithConfig_AllDisabled(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	t.Setenv("OTEL_LOGS_EXPORTER", "none")

	cfg := OTelConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
	}

	ctx := context.Background()
	shutdown, err := SetupOTelSDKWithConfig(ctx, cfg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	err = shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown returned unexpected error: %v", err)
	}
}

// TestSetupOTelSDKWithConfig_ShutdownIdempotent verifies that the shutdown
// function can be called multiple times without error.
func TestSetupOTelSDKWithConfig_ShutdownIdempotent(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	t.Setenv("OTEL_LOGS_EXPORTER", "none")

	cfg := OTelConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
	}

	ctx := context.Background()
	shutdown, err := SetupOTelSDKWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = shutdown(ctx)
	if err != nil {
		t.Errorf("first shutdown returned unexpected error: %v", err)
	}

	// Second call should also succeed (shutdownFuncs is cleared)
	err = shutdown(ctx)
	if err != nil {
		t.Errorf("second shutdown returned unexpected error: %v", err)
	}
}

// TestNewResource verifies that newResource creates a valid OpenTelemetry
// resource with the expected service.name attribute for various input values,
// including edge cases like empty versions and unicode characters.
func TestNewResource(t *testing.T) {
	tests := []struct {
		name           string
		serviceName    string
		serviceVersion string
	}{
		{"basic", "test-service", "1.0.0"},
		{"empty version", "test-service", ""},
		{"unicode name", "测试服务", "2.0.0"},
		{"special chars", "test-service-123", "1.0.0-beta.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := OTelConfig{
				ServiceName:    tt.serviceName,
				ServiceVersion: tt.serviceVersion,
			}

			res, err := newResource(cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if res == nil {
				t.Fatal("expected non-nil resource")
			}

			attrs := res.Attributes()
			foundName := false
			foundVersion := tt.serviceVersion == "" // empty version need not appear
			for _, attr := range attrs {
				if string(attr.Key) == "service.name" && attr.Value.AsString() == tt.serviceName {
					foundName = true
				}
				if tt.serviceVersion != "" && string(attr.Key) == "service.version" && attr.Value.AsString() == tt.serviceVersion {
					foundVersion = true
				}
			}
			if !foundName {
				t.Errorf("resource missing service.name attribute with value %q", tt.serviceName)
			}
			if !foundVersion {
				t.Errorf("resource missing service.version attribute with value %q", tt.serviceVersion)
			}
		})
	}
}

// TestNewSampler verifies that newSampler returns the correct sampler for each
// standard OTEL_TRACES_SAMPLER value.
func TestNewSampler(t *testing.T) {
	tests := []struct {
		sampler     string
		arg         string
		wantDescHas string
	}{
		{"always_on", "", "AlwaysOnSampler"},
		{"always_off", "", "AlwaysOffSampler"},
		{"traceidratio", "0.5", "TraceIDRatioBased{0.5}"},
		{"traceidratio", "", "TraceIDRatioBased{1}"},
		{"parentbased_always_on", "", "ParentBased{root:AlwaysOnSampler"},
		{"parentbased_always_off", "", "ParentBased{root:AlwaysOffSampler"},
		{"unknown_value", "", "ParentBased{root:AlwaysOnSampler"}, // default: parentbased_always_on
		{"", "", "ParentBased{root:AlwaysOnSampler"},              // default: parentbased_always_on
	}

	for _, tt := range tests {
		t.Run(tt.sampler+"_"+tt.arg, func(t *testing.T) {
			tUnsetenv(t, "OTEL_TRACES_SAMPLER")
			tUnsetenv(t, "OTEL_TRACES_SAMPLER_ARG")
			if tt.sampler != "" {
				t.Setenv("OTEL_TRACES_SAMPLER", tt.sampler)
			}
			if tt.arg != "" {
				t.Setenv("OTEL_TRACES_SAMPLER_ARG", tt.arg)
			}

			s := newSampler()
			if s == nil {
				t.Fatal("expected non-nil sampler")
			}
			if !strings.Contains(s.Description(), tt.wantDescHas) {
				t.Errorf("sampler %q: Description() = %q, want substring %q",
					tt.sampler, s.Description(), tt.wantDescHas)
			}
		})
	}
}

// TestNewSampler_InvalidArg verifies that an invalid OTEL_TRACES_SAMPLER_ARG
// falls back to 1.0 (always sample).
func TestNewSampler_InvalidArg(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER", "traceidratio")
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "invalid")

	s := newSampler()
	if s == nil {
		t.Fatal("expected non-nil sampler")
	}
	// With invalid arg, falls back to 1.0 (always sample).
	// TraceIDRatioBased(1.0) describes as "TraceIDRatioBased{1}"
	if s.Description() != "TraceIDRatioBased{1}" {
		t.Errorf("expected TraceIDRatioBased{1}, got %q", s.Description())
	}
}

// TestSetupOTelSDK tests the convenience function SetupOTelSDK which reads
// configuration from environment variables. With all exporters set to "none",
// it should successfully initialize the SDK.
func TestSetupOTelSDK(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	t.Setenv("OTEL_LOGS_EXPORTER", "none")
	tUnsetenv(t, "OTEL_SERVICE_NAME")
	tUnsetenv(t, "OTEL_SERVICE_VERSION")

	ctx := context.Background()
	shutdown, err := SetupOTelSDK(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	err = shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown returned unexpected error: %v", err)
	}
}

// TestOTelConfig_MinimalConfig verifies that the SDK can be initialized with
// a minimal OTelConfig (no service name or version set).
func TestOTelConfig_MinimalConfig(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	t.Setenv("OTEL_LOGS_EXPORTER", "none")

	cfg := OTelConfig{}

	ctx := context.Background()
	shutdown, err := SetupOTelSDKWithConfig(ctx, cfg)

	if err != nil {
		t.Fatalf("unexpected error with minimal config: %v", err)
	}

	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	err = shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown returned unexpected error: %v", err)
	}
}
