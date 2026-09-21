// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/auth0/go-auth0/authentication"
	"github.com/auth0/go-auth0/authentication/oauth"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"
)

const tokenExpiryLeeway = 60 * time.Second

// Config holds ITX proxy configuration
type Config struct {
	BaseURL     string
	Auth0Domain string
	ClientID    string
	PrivateKey  string // RSA private key in PEM format
	Audience    string
	Timeout     time.Duration
}

// Client implements domain.ITXProxyClient
type Client struct {
	httpClient *http.Client
	config     Config
}

// Compile-time assertions: *Client must satisfy all three sub-interfaces and their
// composite. If a method is added to any sub-interface but not implemented here,
// the build fails immediately rather than at runtime.
var (
	_ domain.SurveyClient           = (*Client)(nil)
	_ domain.ExclusionClient        = (*Client)(nil)
	_ domain.SurveyResponseClient   = (*Client)(nil)
	_ domain.ITXProxyClient         = (*Client)(nil)
	_ domain.InviteAcceptanceClient = (*Client)(nil)
)

// auth0TokenSource implements oauth2.TokenSource using Auth0 SDK with private key
type auth0TokenSource struct {
	ctx        context.Context
	authConfig *authentication.Authentication
	audience   string
}

// Token implements the oauth2.TokenSource interface
func (a *auth0TokenSource) Token() (*oauth2.Token, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.TODO()
	}

	// Build and issue a request using Auth0 SDK
	body := oauth.LoginWithClientCredentialsRequest{
		Audience: a.audience,
	}

	tokenSet, err := a.authConfig.OAuth.LoginWithClientCredentials(ctx, body, oauth.IDTokenValidationOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get token from Auth0: %w", err)
	}

	// Convert Auth0 response to oauth2.Token with leeway for expiration
	token := &oauth2.Token{
		AccessToken:  tokenSet.AccessToken,
		TokenType:    tokenSet.TokenType,
		RefreshToken: tokenSet.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tokenSet.ExpiresIn)*time.Second - tokenExpiryLeeway),
	}

	// Add extra fields
	token = token.WithExtra(map[string]any{
		"scope": tokenSet.Scope,
	})

	return token, nil
}

// NewClient creates a new ITX proxy client with OAuth2 M2M authentication using private key
func NewClient(config Config) *Client {
	ctx := context.Background()

	if config.PrivateKey == "" {
		panic("ITX_CLIENT_PRIVATE_KEY is required but not set")
	}

	// Create an otel-instrumented HTTP client for Auth0 token requests;
	// ITX API calls are instrumented separately via httpClient below.
	otelClient := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   config.Timeout,
	}

	// Create Auth0 authentication client with private key assertion (JWT)
	// The private key should be in PEM format (raw, not base64-encoded)
	authConfig, err := authentication.New(
		ctx,
		config.Auth0Domain,
		authentication.WithClientID(config.ClientID),
		authentication.WithClientAssertion(config.PrivateKey, "RS256"),
		authentication.WithClient(otelClient),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create Auth0 client: %v (ensure ITX_CLIENT_PRIVATE_KEY contains a valid RSA private key in PEM format)", err))
	}

	// Create token source
	tokenSource := &auth0TokenSource{
		ctx:        ctx,
		authConfig: authConfig,
		audience:   config.Audience,
	}

	// Wrap with oauth2.ReuseTokenSource for automatic caching and renewal
	reuseTokenSource := oauth2.ReuseTokenSource(nil, tokenSource)

	// Create HTTP client that automatically handles token management.
	// Wrap the oauth2 transport with otelhttp so ITX API calls appear in traces.
	httpClient := oauth2.NewClient(ctx, reuseTokenSource)
	httpClient.Transport = otelhttp.NewTransport(httpClient.Transport)
	httpClient.Timeout = config.Timeout

	return &Client{
		httpClient: httpClient,
		config:     config,
	}
}

// mapHTTPError maps ITX HTTP status codes to domain errors
func (c *Client) mapHTTPError(statusCode int, body []byte) error {
	var errMsg struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}

	// Try to parse JSON error response
	_ = json.Unmarshal(body, &errMsg)

	message := errMsg.Message
	if message == "" {
		message = errMsg.Error
	}
	if message == "" {
		// If no message fields found, include the raw body in the error
		if len(body) > 0 {
			message = fmt.Sprintf("ITX API error: HTTP %d - %s", statusCode, string(body))
		} else {
			message = fmt.Sprintf("ITX API error: HTTP %d", statusCode)
		}
	}

	switch statusCode {
	case http.StatusBadRequest:
		return domain.NewValidationError(message)
	case http.StatusUnauthorized, http.StatusForbidden:
		// There shouldn't be unauthorized or forbidden errors from ITX since we are using M2M authentication,
		// so these errors imply an internal server error due to issues with the M2M credentials.
		return domain.NewInternalError(message)
	case http.StatusNotFound:
		return domain.NewNotFoundError(message)
	case http.StatusConflict:
		return domain.NewConflictError(message)
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return domain.NewUnavailableError(message)
	default:
		return domain.NewInternalError(message)
	}
}

// AcceptInvite calls the ITX survey service to enrich all survey-response records for the
// given email address with the acceptor's username and profile data. This is called after
// a no-LFID participant accepts their invite and gains a username.
func (c *Client) AcceptInvite(ctx context.Context, email, username string) error {
	body, err := json.Marshal(map[string]string{
		"email":    email,
		"username": username,
	})
	if err != nil {
		return domain.NewInternalError("failed to marshal invite_accepted request", err)
	}

	reqURL := fmt.Sprintf("%sv2/surveys/responses/invite_accepted", c.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return domain.NewInternalError("failed to create invite_accepted request", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-scope", "manage:surveys")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX invite_accepted request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read invite_accepted response", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}
