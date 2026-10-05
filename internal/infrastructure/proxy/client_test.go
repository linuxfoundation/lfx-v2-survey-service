// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
)

// testClient builds a minimal Client backed by the given httptest.Server.
// No Auth0 wiring is needed because tests control the server directly.
func testClient(srv *httptest.Server) *Client {
	return &Client{
		httpClient: srv.Client(),
		config:     Config{BaseURL: srv.URL + "/"},
	}
}

// ─── marshalJSON ─────────────────────────────────────────────────────────────

func TestMarshalJSON_ValidValue(t *testing.T) {
	r, err := marshalJSON(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil reader")
	}
}

func TestMarshalJSON_UnmarshalableValue(t *testing.T) {
	_, err := marshalJSON(func() {}) // functions can't be marshalled
	if err == nil {
		t.Fatal("expected error for unmarshalable value")
	}
	var domErr *domain.DomainError
	if !errors.As(err, &domErr) || domErr.Type != domain.ErrorTypeInternal {
		t.Errorf("expected InternalError, got %T: %v", err, err)
	}
}

// ─── doJSON ──────────────────────────────────────────────────────────────────

type testPayload struct {
	Value string `json:"value"`
}

func TestDoJSON_2xxDecodesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(testPayload{Value: "hello"})
	}))
	defer srv.Close()

	c := testClient(srv)
	result, err := doJSON[testPayload](context.Background(), c, http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Value != "hello" {
		t.Errorf("expected 'hello', got %q", result.Value)
	}
}

func TestDoJSON_SetsAcceptHeader(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := testClient(srv)
	_, _ = doJSON[testPayload](context.Background(), c, http.MethodGet, srv.URL+"/test", nil)
	if gotAccept != "application/json" {
		t.Errorf("expected Accept: application/json, got %q", gotAccept)
	}
}

func TestDoJSON_SetsContentTypeOnlyWhenBodyPresent(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := testClient(srv)

	// GET with no body — Content-Type must not be set
	_, _ = doJSON[testPayload](context.Background(), c, http.MethodGet, srv.URL+"/test", nil)
	if gotContentType != "" {
		t.Errorf("GET: expected no Content-Type, got %q", gotContentType)
	}

	// POST with a body — Content-Type must be set
	body := strings.NewReader(`{"key":"val"}`)
	_, _ = doJSON[testPayload](context.Background(), c, http.MethodPost, srv.URL+"/test", body)
	if gotContentType != "application/json" {
		t.Errorf("POST: expected Content-Type: application/json, got %q", gotContentType)
	}
}

func TestDoJSON_Non2xxMapsToHTTPError(t *testing.T) {
	tests := []struct {
		status   int
		wantType domain.ErrorType
	}{
		{http.StatusBadRequest, domain.ErrorTypeValidation},
		{http.StatusNotFound, domain.ErrorTypeNotFound},
		{http.StatusConflict, domain.ErrorTypeConflict},
		{http.StatusServiceUnavailable, domain.ErrorTypeUnavailable},
		{http.StatusInternalServerError, domain.ErrorTypeInternal},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"message":"oops"}`))
			}))
			defer srv.Close()

			c := testClient(srv)
			_, err := doJSON[testPayload](context.Background(), c, http.MethodGet, srv.URL+"/test", nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var domErr *domain.DomainError
			if !errors.As(err, &domErr) {
				t.Fatalf("expected *domain.DomainError, got %T", err)
			}
			if domErr.Type != tt.wantType {
				t.Errorf("status %d: expected ErrorType %v, got %v", tt.status, tt.wantType, domErr.Type)
			}
		})
	}
}

func TestDoJSON_BadResponseJSONReturnsInternalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := testClient(srv)
	_, err := doJSON[testPayload](context.Background(), c, http.MethodGet, srv.URL+"/test", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var domErr *domain.DomainError
	if !errors.As(err, &domErr) || domErr.Type != domain.ErrorTypeInternal {
		t.Errorf("expected InternalError, got %T: %v", err, err)
	}
}

// ─── doNoBody ────────────────────────────────────────────────────────────────

func TestDoNoBody_2xxReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := testClient(srv)
	if err := doNoBody(context.Background(), c, http.MethodDelete, srv.URL+"/test", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoNoBody_Non2xxMapsToHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer srv.Close()

	c := testClient(srv)
	err := doNoBody(context.Background(), c, http.MethodDelete, srv.URL+"/test", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var domErr *domain.DomainError
	if !errors.As(err, &domErr) || domErr.Type != domain.ErrorTypeNotFound {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestDoNoBody_SetsContentTypeOnlyWhenBodyPresent(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := testClient(srv)

	// DELETE with no body
	_ = doNoBody(context.Background(), c, http.MethodDelete, srv.URL+"/test", nil)
	if gotContentType != "" {
		t.Errorf("expected no Content-Type, got %q", gotContentType)
	}

	// POST with a body
	body := strings.NewReader(`{"key":"val"}`)
	_ = doNoBody(context.Background(), c, http.MethodPost, srv.URL+"/test", body)
	if gotContentType != "application/json" {
		t.Errorf("expected Content-Type: application/json, got %q", gotContentType)
	}
}
