// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/auth0/go-auth0/authentication"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// ─── shared operation-table harness ──────────────────────────────────────────

// recordedRequest captures what a proxy method actually put on the wire.
type recordedRequest struct {
	Method      string
	Path        string
	Query       url.Values
	ContentType string
	Scope       string
	Body        []byte
}

// recordingClient returns a Client whose server records every request and
// replies with the given status and body.
func recordingClient(t *testing.T, status int, respBody string) (*Client, *recordedRequest) {
	t.Helper()
	rec := &recordedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.Method = r.Method
		rec.Path = r.URL.Path
		rec.Query = r.URL.Query()
		rec.ContentType = r.Header.Get("Content-Type")
		rec.Scope = r.Header.Get("x-scope")
		rec.Body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return testClient(srv), rec
}

// operation describes one ITX proxy method: the call to make and the HTTP
// request it must produce. runOperations drives every operation through the
// same checks so a new method gets a one-row entry rather than a new test.
type operation struct {
	name   string
	call   func(c *Client) (any, error)
	method string
	path   string
	query  url.Values // nil means no query string at all
	// wantBody is the expected JSON request body; empty means the request must
	// carry no body (and therefore no Content-Type).
	wantBody string
	// respBody is the 2xx JSON body the fake ITX returns; empty means 204.
	respBody string
	// wantResult is the decoded value call must return alongside the response
	// body; nil for methods that discard the response.
	wantResult any
	// check inspects the decoded result when a full-struct comparison would be
	// noise (large ITX payloads); use it to pin the fields that matter.
	check func(t *testing.T, got any)
}

// result adapts a (*T, error) proxy method to the operation call signature.
func result[T any](v *T, err error) (any, error) { return v, err }

// noResult adapts an error-only proxy method to the operation call signature.
func noResult(err error) (any, error) { return nil, err }

func runOperations(t *testing.T, ops []operation) {
	t.Helper()
	for _, op := range ops {
		t.Run(op.name+"/request", func(t *testing.T) {
			status := http.StatusOK
			if op.respBody == "" {
				status = http.StatusNoContent
			}
			c, rec := recordingClient(t, status, op.respBody)

			got, err := op.call(c)
			require.NoError(t, err)

			assert.Equal(t, op.method, rec.Method)
			assert.Equal(t, op.path, rec.Path)
			if op.query == nil {
				assert.Empty(t, rec.Query, "unexpected query parameters")
			} else {
				assert.Equal(t, op.query, rec.Query)
			}
			assert.Equal(t, "manage:surveys", rec.Scope, "ITX scope header")
			if op.wantBody == "" {
				assert.Empty(t, rec.Body)
				assert.Empty(t, rec.ContentType)
			} else {
				assert.JSONEq(t, op.wantBody, string(rec.Body))
				assert.Equal(t, "application/json", rec.ContentType)
			}
			if op.wantResult != nil {
				assert.Equal(t, op.wantResult, got)
			}
			if op.check != nil {
				op.check(t, got)
			}
		})

		t.Run(op.name+"/maps ITX 404 to NotFound", func(t *testing.T) {
			c, _ := recordingClient(t, http.StatusNotFound, `{"message":"missing"}`)
			_, err := op.call(c)
			requireDomainError(t, err, domain.ErrorTypeNotFound)
		})
	}
}

// domainErrorOf asserts err is a *domain.DomainError of the given type and returns it.
func domainErrorOf(t *testing.T, err error, want domain.ErrorType) *domain.DomainError {
	t.Helper()
	require.Error(t, err)
	var domErr *domain.DomainError
	require.True(t, errors.As(err, &domErr), "expected *domain.DomainError, got %T: %v", err, err)
	assert.Equal(t, want, domErr.Type)
	return domErr
}

// requireDomainError asserts err is a *domain.DomainError of the given type.
func requireDomainError(t *testing.T, err error, want domain.ErrorType) {
	t.Helper()
	_ = domainErrorOf(t, err, want)
}

// ─── mapHTTPError ────────────────────────────────────────────────────────────

func TestMapHTTPError(t *testing.T) {
	c := &Client{}
	tests := []struct {
		name        string
		status      int
		body        string
		wantType    domain.ErrorType
		wantMessage string
	}{
		{"400 validation", http.StatusBadRequest, `{"message":"bad"}`, domain.ErrorTypeValidation, "bad"},
		{"401 is internal (M2M credential problem)", http.StatusUnauthorized, `{"message":"expired"}`, domain.ErrorTypeInternal, "expired"},
		{"403 is internal (M2M credential problem)", http.StatusForbidden, `{"message":"denied"}`, domain.ErrorTypeInternal, "denied"},
		{"404 not found", http.StatusNotFound, `{"message":"gone"}`, domain.ErrorTypeNotFound, "gone"},
		{"409 conflict", http.StatusConflict, `{"message":"dup"}`, domain.ErrorTypeConflict, "dup"},
		{"429 unavailable", http.StatusTooManyRequests, `{"message":"slow down"}`, domain.ErrorTypeUnavailable, "slow down"},
		{"503 unavailable", http.StatusServiceUnavailable, `{"message":"down"}`, domain.ErrorTypeUnavailable, "down"},
		{"500 internal", http.StatusInternalServerError, `{"message":"boom"}`, domain.ErrorTypeInternal, "boom"},
		{"unmapped status falls back to internal", http.StatusTeapot, `{"message":"tea"}`, domain.ErrorTypeInternal, "tea"},
		{"error field used when message is empty", http.StatusBadRequest, `{"error":"from error field"}`, domain.ErrorTypeValidation, "from error field"},
		{"message wins over error field", http.StatusBadRequest, `{"message":"m","error":"e"}`, domain.ErrorTypeValidation, "m"},
		{"non-JSON body is echoed", http.StatusBadGateway, `upstream exploded`, domain.ErrorTypeInternal, "ITX API error: HTTP 502 - upstream exploded"},
		{"empty body reports the status only", http.StatusNotFound, ``, domain.ErrorTypeNotFound, "ITX API error: HTTP 404"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.mapHTTPError(tt.status, []byte(tt.body))
			domErr := domainErrorOf(t, err, tt.wantType)
			assert.Equal(t, tt.wantMessage, domErr.Message)
		})
	}
}

// ─── transport and request-construction failures ─────────────────────────────

func TestDoJSON_TransportFailureIsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	c := testClient(srv)
	srv.Close() // nothing listens any more

	_, err := doJSON[testPayload](context.Background(), c, http.MethodGet, srv.URL+"/x", nil)
	requireDomainError(t, err, domain.ErrorTypeUnavailable)
}

func TestDoNoBody_TransportFailureIsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	c := testClient(srv)
	srv.Close()

	err := doNoBody(context.Background(), c, http.MethodDelete, srv.URL+"/x", nil)
	requireDomainError(t, err, domain.ErrorTypeUnavailable)
}

func TestDoJSON_InvalidRequestIsInternal(t *testing.T) {
	c := &Client{httpClient: http.DefaultClient}
	_, err := doJSON[testPayload](context.Background(), c, "BAD METHOD", "http://example.invalid/", nil)
	requireDomainError(t, err, domain.ErrorTypeInternal)
}

func TestDoNoBody_InvalidRequestIsInternal(t *testing.T) {
	c := &Client{httpClient: http.DefaultClient}
	err := doNoBody(context.Background(), c, "BAD METHOD", "http://example.invalid/", nil)
	requireDomainError(t, err, domain.ErrorTypeInternal)
}

func TestDoJSON_CancelledContextIsUnavailable(t *testing.T) {
	c, _ := recordingClient(t, http.StatusOK, `{}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := doJSON[testPayload](ctx, c, http.MethodGet, c.config.BaseURL+"x", nil)
	requireDomainError(t, err, domain.ErrorTypeUnavailable)
}

// ─── AcceptInvite ────────────────────────────────────────────────────────────

func TestClient_AcceptInvite(t *testing.T) {
	runOperations(t, []operation{{
		name: "AcceptInvite",
		call: func(c *Client) (any, error) {
			return noResult(c.AcceptInvite(context.Background(), "alice@example.com", "alice"))
		},
		method:   http.MethodPost,
		path:     "/v2/surveys/responses/invite_accepted",
		wantBody: `{"email":"alice@example.com","username":"alice"}`,
	}})
}

// ─── NewClient and the Auth0 token source ────────────────────────────────────

var (
	testKeyOnce sync.Once
	testKeyPEM  string
)

// testPrivateKeyPEM returns a throwaway RSA key in PEM form for client
// assertions. It is generated once per test binary because 2048-bit keygen is slow.
func testPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	testKeyOnce.Do(func() {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		testKeyPEM = string(pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		}))
	})
	return testKeyPEM
}

func TestNewClient_PanicsWithoutPrivateKey(t *testing.T) {
	assert.PanicsWithValue(t, "ITX_CLIENT_PRIVATE_KEY is required but not set", func() {
		NewClient(Config{Auth0Domain: "tenant.example.com", ClientID: "client"})
	})
}

// fakeTenantTransport stands in for the network: it serves a minimal Auth0
// tenant (JWKS + token endpoint) and a fake ITX host, recording ITX requests.
type fakeTenantTransport struct {
	tokenCalls int
	tokenForm  url.Values // form posted to the last token request
	itxRequest *http.Request
	// tokenStatus/tokenBody override the token endpoint reply when set.
	tokenStatus int
	tokenBody   string
}

func (f *fakeTenantTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	reply := func(status int, body string) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	}
	switch {
	case r.URL.Host == "tenant.example.com" && r.URL.Path == "/.well-known/jwks.json":
		return reply(http.StatusOK, `{"keys":[]}`)
	case r.URL.Host == "tenant.example.com" && r.URL.Path == "/oauth/token":
		f.tokenCalls++
		raw, _ := io.ReadAll(r.Body)
		f.tokenForm, _ = url.ParseQuery(string(raw))
		if f.tokenStatus != 0 {
			return reply(f.tokenStatus, f.tokenBody)
		}
		return reply(http.StatusOK, `{"access_token":"tok-abc","token_type":"Bearer","expires_in":3600,"scope":"manage:surveys"}`)
	case r.URL.Host == "itx.example.com":
		f.itxRequest = r
		return reply(http.StatusOK, `{"id":"excl-1"}`)
	}
	return reply(http.StatusNotFound, `{}`)
}

// An unparseable key is not detected at construction: authentication.New only
// fetches the tenant JWKS, and the key is first parsed when the client assertion
// is signed. So a bad key surfaces as an error on the first ITX call (and no
// request reaches ITX), not as a startup panic.
func TestNewClient_InvalidPrivateKeyFailsOnFirstRequest(t *testing.T) {
	fake := &fakeTenantTransport{}
	useFakeNetwork(t, fake)

	var c *Client
	require.NotPanics(t, func() {
		c = NewClient(Config{
			BaseURL:     "https://itx.example.com/",
			Auth0Domain: "tenant.example.com",
			ClientID:    "client",
			PrivateKey:  "not a pem key",
		})
	})

	_, err := c.GetExclusion(context.Background(), "excl-1")
	require.Error(t, err)
	assert.Nil(t, fake.itxRequest, "no request may reach ITX without a token")
	assert.Zero(t, fake.tokenCalls, "token endpoint is never reached with an unsignable assertion")
}

// useFakeNetwork routes http.DefaultTransport (which NewClient captures) to fake
// for the duration of the test. It mutates a global, so callers must not run in parallel.
func useFakeNetwork(t *testing.T, fake http.RoundTripper) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = fake
	t.Cleanup(func() { http.DefaultTransport = orig })
}

func TestNewClient_BuildsAuthenticatedClient(t *testing.T) {
	fake := &fakeTenantTransport{}
	useFakeNetwork(t, fake)

	cfg := Config{
		BaseURL:     "https://itx.example.com/",
		Auth0Domain: "tenant.example.com",
		ClientID:    "client",
		PrivateKey:  testPrivateKeyPEM(t),
		Audience:    "https://itx.example.com/api",
		Timeout:     7 * time.Second,
	}

	c := NewClient(cfg)

	require.NotNil(t, c)
	assert.Equal(t, cfg, c.config)
	assert.Equal(t, 7*time.Second, c.httpClient.Timeout)

	// A real proxy call must carry the M2M bearer token, and the token must be
	// reused (not re-fetched) across calls.
	for range 2 {
		got, err := c.GetExclusion(context.Background(), "excl-1")
		require.NoError(t, err)
		assert.Equal(t, "excl-1", got.ID)
	}
	require.NotNil(t, fake.itxRequest)
	assert.Equal(t, "Bearer tok-abc", fake.itxRequest.Header.Get("Authorization"))
	assert.Equal(t, "manage:surveys", fake.itxRequest.Header.Get("x-scope"))
	assert.Equal(t, 1, fake.tokenCalls, "token should be cached across calls")
}

// newTokenSource builds an auth0TokenSource whose Auth0 tenant is the fake transport.
func newTokenSource(t *testing.T, fake *fakeTenantTransport, audience string) *auth0TokenSource {
	t.Helper()
	authConfig, err := authentication.New(
		context.Background(),
		"tenant.example.com",
		authentication.WithClientID("client"),
		authentication.WithClientAssertion(testPrivateKeyPEM(t), "RS256"),
		authentication.WithClient(&http.Client{Transport: fake}),
	)
	require.NoError(t, err)
	return &auth0TokenSource{authConfig: authConfig, audience: audience}
}

func TestAuth0TokenSource_Token(t *testing.T) {
	fake := &fakeTenantTransport{}

	before := time.Now()
	token, err := newTokenSource(t, fake, "https://itx.example.com/api").Token()
	require.NoError(t, err)

	assert.Equal(t, "client_credentials", fake.tokenForm.Get("grant_type"))
	assert.Equal(t, "https://itx.example.com/api", fake.tokenForm.Get("audience"))
	assert.Equal(t, "tok-abc", token.AccessToken)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.Equal(t, "manage:surveys", token.Extra("scope"))
	// Expiry is expires_in minus the 60s leeway, so the token is renewed early.
	assert.WithinDuration(t, before.Add(3600*time.Second-60*time.Second), token.Expiry, 5*time.Second)
}

func TestAuth0TokenSource_TokenErrorIsWrapped(t *testing.T) {
	fake := &fakeTenantTransport{
		tokenStatus: http.StatusUnauthorized,
		tokenBody:   `{"error":"access_denied","error_description":"bad assertion"}`,
	}

	token, err := newTokenSource(t, fake, "aud").Token()
	require.Error(t, err)
	assert.Nil(t, token)
	assert.Contains(t, err.Error(), "failed to get token from Auth0")
}
