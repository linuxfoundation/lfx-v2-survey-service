// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	natsgo "github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/propagation"
)

// NatsHeaderCarrier adapts nats.Header to the OTel TextMapCarrier interface
// so trace context can be injected into or extracted from NATS message headers.
// Both the eventing and idmapper infrastructure packages use this adapter.
type NatsHeaderCarrier natsgo.Header

func (c NatsHeaderCarrier) Get(key string) string {
	vals := c[key]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (c NatsHeaderCarrier) Set(key string, value string) {
	if c == nil {
		return
	}
	c[key] = []string{value}
}

func (c NatsHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

var _ propagation.TextMapCarrier = NatsHeaderCarrier{}
