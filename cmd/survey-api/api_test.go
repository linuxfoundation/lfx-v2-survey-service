// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"text/template"

	surveysvr "github.com/linuxfoundation/lfx-v2-survey-service/gen/http/survey/server"
	"github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/infrastructure/idmapper"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/middleware"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goahttp "goa.design/goa/v3/http"
	"goa.design/goa/v3/security"
	"gopkg.in/yaml.v3"
)

func TestMapDomainError_NonDomainError(t *testing.T) {
	err := mapDomainError(context.Background(), errors.New("plain error"))
	if _, ok := err.(*survey.InternalServerError); !ok {
		t.Errorf("expected *survey.InternalServerError, got %T", err)
	}
}

func TestMapDomainError_EachErrorType(t *testing.T) {
	tests := []struct {
		errType  domain.ErrorType
		msg      string
		wantCode string
		wantType interface{}
	}{
		{
			errType:  domain.ErrorTypeValidation,
			msg:      "bad input",
			wantCode: "400",
			wantType: (*survey.BadRequestError)(nil),
		},
		{
			errType:  domain.ErrorTypeNotFound,
			msg:      "not found",
			wantCode: "404",
			wantType: (*survey.NotFoundError)(nil),
		},
		{
			errType:  domain.ErrorTypeConflict,
			msg:      "conflict",
			wantCode: "409",
			wantType: (*survey.ConflictError)(nil),
		},
		{
			errType:  domain.ErrorTypeUnavailable,
			msg:      "unavailable",
			wantCode: "503",
			wantType: (*survey.ServiceUnavailableError)(nil),
		},
		{
			errType:  domain.ErrorTypeInternal,
			msg:      "internal",
			wantCode: "500",
			wantType: (*survey.InternalServerError)(nil),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			domErr := &domain.DomainError{Type: tt.errType, Message: tt.msg}
			got := mapDomainError(context.Background(), domErr)

			// Verify correct Goa type
			switch tt.wantType.(type) {
			case *survey.BadRequestError:
				e, ok := got.(*survey.BadRequestError)
				if !ok {
					t.Fatalf("expected *survey.BadRequestError, got %T", got)
				}
				if e.Code != tt.wantCode {
					t.Errorf("code: want %s, got %s", tt.wantCode, e.Code)
				}
				if e.Message != tt.msg {
					t.Errorf("message: want %q, got %q", tt.msg, e.Message)
				}
			case *survey.NotFoundError:
				e, ok := got.(*survey.NotFoundError)
				if !ok {
					t.Fatalf("expected *survey.NotFoundError, got %T", got)
				}
				if e.Code != tt.wantCode {
					t.Errorf("code: want %s, got %s", tt.wantCode, e.Code)
				}
			case *survey.ConflictError:
				e, ok := got.(*survey.ConflictError)
				if !ok {
					t.Fatalf("expected *survey.ConflictError, got %T", got)
				}
				if e.Code != tt.wantCode {
					t.Errorf("code: want %s, got %s", tt.wantCode, e.Code)
				}
			case *survey.ServiceUnavailableError:
				e, ok := got.(*survey.ServiceUnavailableError)
				if !ok {
					t.Fatalf("expected *survey.ServiceUnavailableError, got %T", got)
				}
				if e.Code != tt.wantCode {
					t.Errorf("code: want %s, got %s", tt.wantCode, e.Code)
				}
			case *survey.InternalServerError:
				e, ok := got.(*survey.InternalServerError)
				if !ok {
					t.Fatalf("expected *survey.InternalServerError, got %T", got)
				}
				if e.Code != tt.wantCode {
					t.Errorf("code: want %s, got %s", tt.wantCode, e.Code)
				}
			}
		})
	}
}

// captureDefaultLogger routes the default slog logger into a buffer at Debug
// level for the duration of the test. Tests using it must not run in parallel.
func captureDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func decodeLogRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	dec := json.NewDecoder(buf)
	for dec.More() {
		var rec map[string]any
		require.NoError(t, dec.Decode(&rec))
		records = append(records, rec)
	}
	return records
}

// TestMapDomainError_LogsOnce covers mapDomainError when nothing tracks the request
// (a handler called directly): it logs the failure itself, once, using the status the
// error type maps to. Under RequestLoggerMiddleware the middleware logs instead, with
// the status actually sent; see TestRequestFailureLog_MatchesWireStatus.
func TestMapDomainError_LogsOnce(t *testing.T) {
	cause := errors.New("dial tcp 10.0.0.1:4222: i/o timeout")

	tests := []struct {
		name        string
		err         error
		wantLevel   string
		wantStatus  float64
		wantInError []string
	}{
		{"plain error is a 500", errors.New("itx exploded"), "ERROR", 500, []string{"itx exploded"}},
		{"validation is a 400", domain.NewValidationError("bad uid"), "DEBUG", 400, []string{"bad uid"}},
		{"not found is a 404", domain.NewNotFoundError("no such survey"), "DEBUG", 404, []string{"no such survey"}},
		{"conflict is a 409", domain.NewConflictError("already exists"), "DEBUG", 409, []string{"already exists"}},
		{"unavailable is a 503", domain.NewUnavailableError("itx down", cause), "ERROR", 503, []string{"itx down", "i/o timeout"}},
		{"internal is a 500", domain.NewInternalError("boom", cause), "ERROR", 500, []string{"boom", "i/o timeout"}},
		{
			"context added by the service survives the wrap",
			fmt.Errorf("map committee_uid %q to v1: %w", "committee-1", domain.NewUnavailableError("lookup timed out", cause)),
			"ERROR", 503,
			[]string{`map committee_uid "committee-1" to v1`, "lookup timed out", "i/o timeout"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureDefaultLogger(t)

			got := mapDomainError(context.Background(), tt.err)

			assert.Error(t, got, "the failure must still reach the client")
			records := decodeLogRecords(t, buf)
			require.Len(t, records, 1, "a failed request must be logged exactly once")
			assert.Equal(t, "request failed", records[0]["msg"])
			assert.Equal(t, tt.wantLevel, records[0]["level"])
			assert.Equal(t, tt.wantStatus, records[0]["status"])
			for _, want := range tt.wantInError {
				assert.Contains(t, records[0]["error"], want)
			}
		})
	}

	t.Run("the client response omits the wrapped cause", func(t *testing.T) {
		captureDefaultLogger(t)

		got := mapDomainError(context.Background(), domain.NewUnavailableError("itx down", cause))

		var unavailable *survey.ServiceUnavailableError
		require.ErrorAs(t, got, &unavailable)
		assert.Equal(t, "itx down", unavailable.Message)
		assert.NotContains(t, unavailable.Message, "i/o timeout")
	})
}

// TestMapDomainError_RecordsForRequestLogger checks that under RequestLoggerMiddleware
// mapDomainError records the failure instead of logging it, so the failure is logged once,
// by the middleware, with the status the response actually carries.
func TestMapDomainError_RecordsForRequestLogger(t *testing.T) {
	buf := captureDefaultLogger(t)
	handler := middleware.RequestLoggerMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Error(t, mapDomainError(r.Context(), domain.NewNotFoundError("no such survey")), "the client still gets an error")
		// The transport decides the status; it need not match the error type's own.
		w.WriteHeader(http.StatusInternalServerError)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/surveys/s-1", nil))

	failures := requestFailureRecords(decodeLogRecords(t, buf))
	require.Len(t, failures, 1)
	assert.Equal(t, float64(http.StatusInternalServerError), failures[0]["status"], "the sent status, not the mapped 404")
	assert.Equal(t, "ERROR", failures[0]["level"])
}

func requestFailureRecords(records []map[string]any) []map[string]any {
	var out []map[string]any
	for _, r := range records {
		if r["msg"] == "request failed" {
			out = append(out, r)
		}
	}
	return out
}

// serveRoutingJSON sends a request with an optional JSON body through h.
func serveRoutingJSON(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestRequestFailureLog_MatchesWireStatus drives every endpoint with every kind of error
// through RequestLoggerMiddleware and the generated HTTP stack, and checks that the
// failure is logged exactly once, with the status the client actually receives and the
// level that status calls for. The design declares a different set of error responses on
// each endpoint, and a generated encoder sends one it does not declare as a generic 500,
// so the status cannot be derived from the error type.
func TestRequestFailureLog_MatchesWireStatus(t *testing.T) {
	const (
		surveyUID   = "b03cdbaf-53b1-4d47-bc04-dd7e459dd309"
		responseID  = "r-1"
		exclusionID = "e-1"
		projectUID  = "7cad5a8d-19d0-41a4-81a6-043453daf9ee"
	)
	endpoints := []struct {
		name, method, path, body string
		wire                     func(c *handlerClients, err error)
	}{
		{"ScheduleSurvey", "POST", "/surveys", `{"committee_uid":"c-1"}`, func(c *handlerClients, err error) {
			c.survey.ScheduleSurveyFunc = func(context.Context, *itx.ScheduleSurveyRequest) (*itx.SurveyScheduleResponse, error) {
				return nil, err
			}
		}},
		{"GetSurvey", "GET", "/surveys/" + surveyUID, "", func(c *handlerClients, err error) {
			c.survey.GetSurveyFunc = func(context.Context, string, *itx.GetSurveyParams) (*itx.SurveyScheduleResponse, error) {
				return nil, err
			}
		}},
		{"UpdateSurvey", "PUT", "/surveys/" + surveyUID, `{}`, func(c *handlerClients, err error) {
			c.survey.UpdateSurveyFunc = func(context.Context, string, *itx.UpdateSurveyRequest) (*itx.SurveyScheduleResponse, error) {
				return nil, err
			}
		}},
		{"DeleteSurvey", "DELETE", "/surveys/" + surveyUID, "", func(c *handlerClients, err error) {
			c.survey.DeleteSurveyFunc = func(context.Context, string) error { return err }
		}},
		{"BulkResendSurvey", "POST", "/surveys/" + surveyUID + "/bulk_resend", `{"recipient_ids":["r-1"]}`, func(c *handlerClients, err error) {
			c.survey.BulkResendSurveyFunc = func(context.Context, string, *itx.BulkResendRequest) error { return err }
		}},
		{"PreviewSendSurvey", "GET", "/surveys/" + surveyUID + "/preview_send", "", func(c *handlerClients, err error) {
			c.survey.PreviewSendFunc = func(context.Context, string, *string) (*itx.PreviewSendResponse, error) { return nil, err }
		}},
		{"SendMissingRecipients", "POST", "/surveys/" + surveyUID + "/send_missing_recipients", "", func(c *handlerClients, err error) {
			c.survey.SendMissingRecipientsFunc = func(context.Context, string, *string) error { return err }
		}},
		{"DeleteSurveyResponse", "DELETE", "/surveys/" + surveyUID + "/responses/" + responseID, "", func(c *handlerClients, err error) {
			c.response.DeleteResponseFunc = func(context.Context, string, string) error { return err }
		}},
		{"ResendSurveyResponse", "POST", "/surveys/" + surveyUID + "/responses/" + responseID + "/resend", "", func(c *handlerClients, err error) {
			c.response.ResendResponseFunc = func(context.Context, string, string) error { return err }
		}},
		{"DeleteRecipientGroup", "DELETE", "/surveys/" + surveyUID + "/recipient_group", "", func(c *handlerClients, err error) {
			c.survey.DeleteRecipientGroupFunc = func(context.Context, string, *string, *string, *string) error { return err }
		}},
		{"CreateExclusion", "POST", "/surveys/exclusion", `{"email":"alice@example.com"}`, func(c *handlerClients, err error) {
			c.exclusion.CreateExclusionFunc = func(context.Context, *itx.ExclusionRequest) (*itx.Exclusion, error) { return nil, err }
		}},
		{"DeleteExclusion", "DELETE", "/surveys/exclusion", `{"email":"alice@example.com"}`, func(c *handlerClients, err error) {
			c.exclusion.DeleteExclusionFunc = func(context.Context, *itx.ExclusionRequest) error { return err }
		}},
		{"GetExclusion", "GET", "/surveys/exclusion/" + exclusionID, "", func(c *handlerClients, err error) {
			c.exclusion.GetExclusionFunc = func(context.Context, string) (*itx.ExtendedExclusion, error) { return nil, err }
		}},
		{"DeleteExclusionByID", "DELETE", "/surveys/exclusion/" + exclusionID, "", func(c *handlerClients, err error) {
			c.exclusion.DeleteExclusionByIDFunc = func(context.Context, string) error { return err }
		}},
		{"ListSurveyResponses", "GET", "/surveys/" + surveyUID + "/responses", "", func(c *handlerClients, err error) {
			c.response.ListResponsesFunc = func(context.Context, string, *itx.ListResponsesParams) (*itx.PaginatedSurveyResponses, error) {
				return nil, err
			}
		}},
		{"ListSurveyTemplates", "GET", "/surveys/templates?project_uid=" + projectUID + "&committee_uid=" + surveyUID, "", func(c *handlerClients, err error) {
			c.survey.ListSurveyTemplatesFunc = func(context.Context) ([]itx.SurveyTemplate, error) { return nil, err }
		}},
		{"ValidateEmail", "POST", "/surveys/validate_email", `{"body":"b","subject":"s"}`, func(c *handlerClients, err error) {
			c.survey.ValidateEmailFunc = func(context.Context, *itx.ValidateEmailRequest) (*itx.ValidateEmailResponse, error) {
				return nil, err
			}
		}},
	}

	failures := []struct {
		name string
		err  error
	}{
		{"validation", domain.NewValidationError("bad input")},
		{"not found", domain.NewNotFoundError("no such thing")},
		{"conflict", domain.NewConflictError("already exists")},
		{"unavailable", domain.NewUnavailableError("itx down")},
		{"internal", domain.NewInternalError("boom")},
		{"plain error", errors.New("itx exploded")},
	}

	for _, ep := range endpoints {
		for _, f := range failures {
			t.Run(ep.name+"/"+f.name, func(t *testing.T) {
				buf := captureDefaultLogger(t)
				c := &handlerClients{
					survey:    &mocks.MockSurveyClient{},
					exclusion: &mocks.MockExclusionClient{},
					response:  &mocks.MockSurveyResponseClient{},
				}
				ep.wire(c, f.err)
				h := middleware.RequestLoggerMiddleware()(newRoutingTestHandlerWith(c.survey, c.exclusion, c.response))

				rec := serveRoutingJSON(t, h, ep.method, ep.path, ep.body)

				require.GreaterOrEqual(t, rec.Code, 400, "expected a failure response, got %d: %s", rec.Code, rec.Body.String())
				records := requestFailureRecords(decodeLogRecords(t, buf))
				require.Len(t, records, 1, "the failure must be logged exactly once (response %d: %s)", rec.Code, rec.Body.String())
				assert.Equal(t, float64(rec.Code), records[0]["status"], "logged status must equal the status sent")
				wantLevel := "DEBUG"
				if rec.Code >= http.StatusInternalServerError {
					wantLevel = "ERROR"
				}
				assert.Equal(t, wantLevel, records[0]["level"])
			})
		}
	}
}

// newRoutingTestHandler mounts the generated survey HTTP server the same way
// main.go does, backed by the given survey client mock.
func newRoutingTestHandler(client *mocks.MockSurveyClient) http.Handler {
	return newRoutingTestHandlerWith(client, &mocks.MockExclusionClient{}, &mocks.MockSurveyResponseClient{})
}

// newRoutingTestHandlerWith is newRoutingTestHandler with every ITX client mock supplied.
func newRoutingTestHandlerWith(client *mocks.MockSurveyClient, exclusion *mocks.MockExclusionClient, response *mocks.MockSurveyResponseClient) http.Handler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	surveyService := service.NewSurveyService(client, exclusion, response, idmapper.NewNoOpMapper(), logger)
	surveyAPI := NewSurveyAPI(surveyService, &mocks.MockAuthenticator{Principal: "test-user"})

	mux := goahttp.NewMuxer()
	surveyServer := surveysvr.New(survey.NewEndpoints(surveyAPI), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	surveysvr.Mount(mux, surveyServer)
	return mux
}

func serveRoutingRequest(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestListSurveyTemplatesRoute_ReturnsTemplates(t *testing.T) {
	h := newRoutingTestHandler(&mocks.MockSurveyClient{
		ListSurveyTemplatesFunc: func(ctx context.Context) ([]itx.SurveyTemplate, error) {
			return []itx.SurveyTemplate{
				{ID: "412349483", Title: "Board Member NPS", Nickname: "Board NPS", Href: "https://api.surveymonkey.com/v3/surveys/412349483"},
			}, nil
		},
	})

	rec := serveRoutingRequest(t, h, "/surveys/templates?project_uid=7cad5a8d-19d0-41a4-81a6-043453daf9ee&committee_uid=b03cdbaf-53b1-4d47-bc04-dd7e459dd309")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body []map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON array body, got %q: %v", rec.Body.String(), err)
	}
	if len(body) != 1 {
		t.Fatalf("expected 1 template, got %d", len(body))
	}
	want := map[string]string{
		"id":       "412349483",
		"title":    "Board Member NPS",
		"nickname": "Board NPS",
		"href":     "https://api.surveymonkey.com/v3/surveys/412349483",
	}
	for k, v := range want {
		if body[0][k] != v {
			t.Errorf("expected %s=%q, got %q", k, v, body[0][k])
		}
	}
}

func TestListSurveyTemplatesRoute_InvalidQueryReturnsBadRequest(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{name: "missing project_uid", target: "/surveys/templates"},
		{name: "invalid project_uid", target: "/surveys/templates?project_uid=not-a-uuid"},
		{name: "invalid committee_uid", target: "/surveys/templates?project_uid=7cad5a8d-19d0-41a4-81a6-043453daf9ee&committee_uid=not-a-uuid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// An unset ListSurveyTemplatesFunc panics, so reaching ITX fails the test.
			h := newRoutingTestHandler(&mocks.MockSurveyClient{})

			rec := serveRoutingRequest(t, h, tt.target)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestGetSurveyRoute_StillResolvesSurveyUID(t *testing.T) {
	var gotSurveyID string
	h := newRoutingTestHandler(&mocks.MockSurveyClient{
		GetSurveyFunc: func(ctx context.Context, surveyID string, _ *itx.GetSurveyParams) (*itx.SurveyScheduleResponse, error) {
			gotSurveyID = surveyID
			return nil, domain.NewNotFoundError("survey not found")
		},
	})

	rec := serveRoutingRequest(t, h, "/surveys/b03cdbaf-53b1-4d47-bc04-dd7e459dd309")
	if gotSurveyID != "b03cdbaf-53b1-4d47-bc04-dd7e459dd309" {
		t.Errorf("expected get_survey to receive the survey UID, got %q", gotSurveyID)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 from mocked get_survey, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestListSurveyTemplatesRule_UsesScalarQueryUIDs exercises the chart and runtime
// Go templates; HTTP routing tests alone bypass Heimdall authorization.
func TestListSurveyTemplatesRule_UsesScalarQueryUIDs(t *testing.T) {
	source, err := os.ReadFile("../../charts/lfx-v2-survey-service/templates/ruleset.yaml")
	if err != nil {
		t.Fatal(err)
	}
	render := func(expression string, data any) string {
		t.Helper()
		tmpl, err := template.New("ruleset").Parse(expression)
		if err != nil {
			t.Fatal(err)
		}
		var result bytes.Buffer
		if err := tmpl.Execute(&result, data); err != nil {
			t.Fatal(err)
		}
		return result.String()
	}
	chart := map[string]any{
		"Chart": map[string]any{"Name": "lfx-v2-survey-service"},
		"Values": map[string]any{
			"heimdall": map[string]any{"enabled": true},
			"openfga":  map[string]any{"enabled": true},
			"app":      map[string]any{"use_oidc_contextualizer": false, "audience": "survey"},
			"lfx":      map[string]any{"namespace": "lfx"},
		},
	}
	var document struct {
		Spec struct {
			Rules []struct {
				ID      string `yaml:"id"`
				Execute []struct {
					Authorizer string `yaml:"authorizer"`
					Config     struct {
						Values map[string]string `yaml:"values"`
					} `yaml:"config"`
				} `yaml:"execute"`
			} `yaml:"rules"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal([]byte(render(string(source), chart)), &document); err != nil {
		t.Fatal(err)
	}
	var values map[string]string
	for _, rule := range document.Spec.Rules {
		if rule.ID == "rule:lfx:lfx-v2-survey-service:surveys:templates:list" {
			for _, step := range rule.Execute {
				if step.Authorizer == "openfga_or_check" {
					values = step.Config.Values
				}
			}
		}
	}
	if values == nil {
		t.Fatal("template-list writer authorization rule not found")
	}
	if values["relation"] != "writer_guard" {
		t.Fatalf("expected primary relation writer_guard, got %q", values["relation"])
	}

	const projectUID = "7cad5a8d-19d0-41a4-81a6-043453daf9ee"
	const committeeUID = "b03cdbaf-53b1-4d47-bc04-dd7e459dd309"
	for _, tt := range []struct {
		name          string
		query         string
		wantSecondary string
	}{
		{name: "project writer", query: "project_uid=" + projectUID},
		{name: "committee writer", query: "project_uid=" + projectUID + "&committee_uid=" + committeeUID, wantSecondary: "committee:" + committeeUID},
		{name: "empty optional committee", query: "project_uid=" + projectUID + "&committee_uid="},
		{name: "first project value matches Goa", query: "project_uid=" + projectUID + "&project_uid=" + committeeUID},
		{name: "first committee value matches Goa", query: "project_uid=" + projectUID + "&committee_uid=" + committeeUID + "&committee_uid=" + projectUID, wantSecondary: "committee:" + committeeUID},
		{name: "first empty committee stays absent", query: "project_uid=" + projectUID + "&committee_uid=&committee_uid=" + committeeUID},
	} {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse("https://example.test/surveys/templates?" + tt.query)
			if err != nil {
				t.Fatal(err)
			}
			request := map[string]any{"Request": map[string]any{"URL": u}}
			if got := render(values["object"], request); got != "project:"+projectUID {
				t.Errorf("primary object: got %q, want %q", got, "project:"+projectUID)
			}
			if got := render(values["or_object"], request); got != tt.wantSecondary {
				t.Errorf("secondary object: got %q, want %q", got, tt.wantSecondary)
			}
			wantRelation := ""
			if tt.wantSecondary != "" {
				wantRelation = "writer"
			}
			if got := render(values["or_relation"], request); got != wantRelation {
				t.Errorf("secondary relation: got %q, want %q", got, wantRelation)
			}
		})
	}
}

func TestJWTAuth(t *testing.T) {
	t.Run("stores the authenticated principal in the context", func(t *testing.T) {
		api := NewSurveyAPI(nil, &mocks.MockAuthenticator{Principal: "alice"})

		ctx, err := api.JWTAuth(context.Background(), "token", &security.JWTScheme{})

		assert.NoError(t, err)
		assert.Equal(t, "alice", ctx.Value(constants.PrincipalContextID))
	})

	t.Run("maps an authentication failure to an unauthorized error", func(t *testing.T) {
		api := NewSurveyAPI(nil, &mocks.MockAuthenticator{Err: errors.New("token expired")})

		ctx, err := api.JWTAuth(context.Background(), "token", &security.JWTScheme{})

		var unauthorized *survey.UnauthorizedError
		if assert.ErrorAs(t, err, &unauthorized) {
			assert.Equal(t, "401", unauthorized.Code)
			assert.Equal(t, "Unauthorized: token expired", unauthorized.Message)
		}
		assert.Nil(t, ctx.Value(constants.PrincipalContextID), "principal must not be set on failure")
	})
}

// handlerClients holds the mocked ITX clients behind a SurveyAPI under test.
type handlerClients struct {
	survey    *mocks.MockSurveyClient
	exclusion *mocks.MockExclusionClient
	response  *mocks.MockSurveyResponseClient
}

// TestSurveyAPI_Handlers verifies that every Goa handler adapter forwards to the
// service, returns the service result untouched on success, and translates a
// domain error into the matching Goa transport error.
func TestSurveyAPI_Handlers(t *testing.T) {
	sp := func(s string) *string { return &s }
	notFound := &domain.DomainError{Type: domain.ErrorTypeNotFound, Message: "itx says missing"}

	// Each case wires one client method to return err (nil means success) and
	// calls one handler. check inspects the success result; void handlers leave
	// it nil and are expected to return a nil result.
	tests := []struct {
		name  string
		wire  func(c *handlerClients, err error)
		call  func(ctx context.Context, api *SurveyAPI) (any, error)
		check func(t *testing.T, got any)
	}{
		{
			name: "ScheduleSurvey",
			wire: func(c *handlerClients, err error) {
				c.survey.ScheduleSurveyFunc = func(context.Context, *itx.ScheduleSurveyRequest) (*itx.SurveyScheduleResponse, error) {
					if err != nil {
						return nil, err
					}
					return &itx.SurveyScheduleResponse{ID: "survey-schedule-1", SurveyTitle: sp("Q4 Survey")}, nil
				}
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return api.ScheduleSurvey(ctx, &survey.ScheduleSurveyPayload{CommitteeUID: "committee-1"})
			},
			check: func(t *testing.T, got any) {
				res := got.(*survey.SurveyScheduleResult)
				assert.Equal(t, "survey-schedule-1", res.UID)
				assert.Equal(t, "Q4 Survey", *res.SurveyTitle)
			},
		},
		{
			name: "GetSurvey",
			wire: func(c *handlerClients, err error) {
				c.survey.GetSurveyFunc = func(context.Context, string, *itx.GetSurveyParams) (*itx.SurveyScheduleResponse, error) {
					if err != nil {
						return nil, err
					}
					return &itx.SurveyScheduleResponse{ID: "survey-get-1"}, nil
				}
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return api.GetSurvey(ctx, &survey.GetSurveyPayload{SurveyUID: "survey-get-1"})
			},
			check: func(t *testing.T, got any) {
				assert.Equal(t, "survey-get-1", got.(*survey.SurveyScheduleResult).UID)
			},
		},
		{
			name: "UpdateSurvey",
			wire: func(c *handlerClients, err error) {
				c.survey.UpdateSurveyFunc = func(context.Context, string, *itx.UpdateSurveyRequest) (*itx.SurveyScheduleResponse, error) {
					if err != nil {
						return nil, err
					}
					return &itx.SurveyScheduleResponse{ID: "survey-update-1"}, nil
				}
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return api.UpdateSurvey(ctx, &survey.UpdateSurveyPayload{SurveyUID: "survey-update-1"})
			},
			check: func(t *testing.T, got any) {
				assert.Equal(t, "survey-update-1", got.(*survey.SurveyScheduleResult).UID)
			},
		},
		{
			name: "DeleteSurvey",
			wire: func(c *handlerClients, err error) {
				c.survey.DeleteSurveyFunc = func(context.Context, string) error { return err }
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return nil, api.DeleteSurvey(ctx, &survey.DeleteSurveyPayload{SurveyUID: "survey-1"})
			},
		},
		{
			name: "BulkResendSurvey",
			wire: func(c *handlerClients, err error) {
				c.survey.BulkResendSurveyFunc = func(context.Context, string, *itx.BulkResendRequest) error { return err }
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return nil, api.BulkResendSurvey(ctx, &survey.BulkResendSurveyPayload{SurveyUID: "survey-1", RecipientIds: []string{"recipient-1"}})
			},
		},
		{
			name: "PreviewSendSurvey",
			wire: func(c *handlerClients, err error) {
				c.survey.PreviewSendFunc = func(context.Context, string, *string) (*itx.PreviewSendResponse, error) {
					if err != nil {
						return nil, err
					}
					return &itx.PreviewSendResponse{AffectedProjects: []itx.LFXProject{{ID: "project-1", Name: "Test Project"}}}, nil
				}
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return api.PreviewSendSurvey(ctx, &survey.PreviewSendSurveyPayload{SurveyUID: "survey-1"})
			},
			check: func(t *testing.T, got any) {
				res := got.(*survey.PreviewSendResult)
				require.Len(t, res.AffectedProjects, 1)
				assert.Equal(t, "project-1", res.AffectedProjects[0].ID)
			},
		},
		{
			name: "SendMissingRecipients",
			wire: func(c *handlerClients, err error) {
				c.survey.SendMissingRecipientsFunc = func(context.Context, string, *string) error { return err }
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return nil, api.SendMissingRecipients(ctx, &survey.SendMissingRecipientsPayload{SurveyUID: "survey-1"})
			},
		},
		{
			name: "DeleteSurveyResponse",
			wire: func(c *handlerClients, err error) {
				c.response.DeleteResponseFunc = func(context.Context, string, string) error { return err }
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return nil, api.DeleteSurveyResponse(ctx, &survey.DeleteSurveyResponsePayload{SurveyUID: "survey-1", ResponseID: "response-1"})
			},
		},
		{
			name: "ResendSurveyResponse",
			wire: func(c *handlerClients, err error) {
				c.response.ResendResponseFunc = func(context.Context, string, string) error { return err }
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return nil, api.ResendSurveyResponse(ctx, &survey.ResendSurveyResponsePayload{SurveyUID: "survey-1", ResponseID: "response-1"})
			},
		},
		{
			name: "DeleteRecipientGroup",
			wire: func(c *handlerClients, err error) {
				c.survey.DeleteRecipientGroupFunc = func(context.Context, string, *string, *string, *string) error { return err }
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return nil, api.DeleteRecipientGroup(ctx, &survey.DeleteRecipientGroupPayload{SurveyUID: "survey-1", CommitteeUID: sp("committee-1")})
			},
		},
		{
			name: "CreateExclusion",
			wire: func(c *handlerClients, err error) {
				c.exclusion.CreateExclusionFunc = func(context.Context, *itx.ExclusionRequest) (*itx.Exclusion, error) {
					if err != nil {
						return nil, err
					}
					return &itx.Exclusion{ID: "exclusion-create-1", Email: sp("alice@example.com")}, nil
				}
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return api.CreateExclusion(ctx, &survey.CreateExclusionPayload{Email: sp("alice@example.com")})
			},
			check: func(t *testing.T, got any) {
				res := got.(*survey.ExclusionResult)
				assert.Equal(t, "exclusion-create-1", res.UID)
				assert.Equal(t, "alice@example.com", *res.Email)
			},
		},
		{
			name: "DeleteExclusion",
			wire: func(c *handlerClients, err error) {
				c.exclusion.DeleteExclusionFunc = func(context.Context, *itx.ExclusionRequest) error { return err }
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return nil, api.DeleteExclusion(ctx, &survey.DeleteExclusionPayload{Email: sp("bob@example.com")})
			},
		},
		{
			name: "GetExclusion",
			wire: func(c *handlerClients, err error) {
				c.exclusion.GetExclusionFunc = func(context.Context, string) (*itx.ExtendedExclusion, error) {
					if err != nil {
						return nil, err
					}
					return &itx.ExtendedExclusion{Exclusion: itx.Exclusion{ID: "exclusion-get-1"}}, nil
				}
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return api.GetExclusion(ctx, &survey.GetExclusionPayload{ExclusionID: "exclusion-get-1"})
			},
			check: func(t *testing.T, got any) {
				assert.Equal(t, "exclusion-get-1", got.(*survey.ExtendedExclusionResult).UID)
			},
		},
		{
			name: "DeleteExclusionByID",
			wire: func(c *handlerClients, err error) {
				c.exclusion.DeleteExclusionByIDFunc = func(context.Context, string) error { return err }
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return nil, api.DeleteExclusionByID(ctx, &survey.DeleteExclusionByIDPayload{ExclusionID: "exclusion-1"})
			},
		},
		{
			name: "ListSurveyResponses",
			wire: func(c *handlerClients, err error) {
				c.response.ListResponsesFunc = func(context.Context, string, *itx.ListResponsesParams) (*itx.PaginatedSurveyResponses, error) {
					if err != nil {
						return nil, err
					}
					return &itx.PaginatedSurveyResponses{Meta: itx.PageMetadata{PageToken: "page-token-1"}}, nil
				}
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return api.ListSurveyResponses(ctx, &survey.ListSurveyResponsesPayload{SurveyUID: "survey-1"})
			},
			check: func(t *testing.T, got any) {
				assert.Equal(t, "page-token-1", *got.(*survey.SurveyResponsesPage).Meta.PageToken)
			},
		},
		{
			name: "ListSurveyTemplates",
			wire: func(c *handlerClients, err error) {
				c.survey.ListSurveyTemplatesFunc = func(context.Context) ([]itx.SurveyTemplate, error) {
					if err != nil {
						return nil, err
					}
					return []itx.SurveyTemplate{{ID: "template-1"}}, nil
				}
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return api.ListSurveyTemplates(ctx, &survey.ListSurveyTemplatesPayload{})
			},
			check: func(t *testing.T, got any) {
				res := got.([]*survey.SurveyTemplate)
				require.Len(t, res, 1)
				assert.Equal(t, "template-1", res[0].ID)
			},
		},
		{
			name: "ValidateEmail",
			wire: func(c *handlerClients, err error) {
				c.survey.ValidateEmailFunc = func(context.Context, *itx.ValidateEmailRequest) (*itx.ValidateEmailResponse, error) {
					if err != nil {
						return nil, err
					}
					return &itx.ValidateEmailResponse{Body: "validated body", Subject: "validated subject"}, nil
				}
			},
			call: func(ctx context.Context, api *SurveyAPI) (any, error) {
				return api.ValidateEmail(ctx, &survey.ValidateEmailPayload{Body: sp("body"), Subject: sp("subject")})
			},
			check: func(t *testing.T, got any) {
				res := got.(*survey.ValidateEmailResult)
				assert.Equal(t, "validated body", res.Body)
				assert.Equal(t, "validated subject", res.Subject)
			},
		},
	}

	newAPI := func(wire func(c *handlerClients)) *SurveyAPI {
		c := &handlerClients{
			survey:    &mocks.MockSurveyClient{},
			exclusion: &mocks.MockExclusionClient{},
			response:  &mocks.MockSurveyResponseClient{},
		}
		wire(c)
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		svc := service.NewSurveyService(c.survey, c.exclusion, c.response, idmapper.NewNoOpMapper(), logger)
		return NewSurveyAPI(svc, &mocks.MockAuthenticator{Principal: "alice"})
	}
	ctx := context.WithValue(context.Background(), constants.PrincipalContextID, "alice")

	for _, tt := range tests {
		t.Run(tt.name+"/returns service result", func(t *testing.T) {
			api := newAPI(func(c *handlerClients) { tt.wire(c, nil) })

			got, err := tt.call(ctx, api)

			require.NoError(t, err)
			if tt.check == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			tt.check(t, got)
		})

		t.Run(tt.name+"/maps domain error", func(t *testing.T) {
			api := newAPI(func(c *handlerClients) { tt.wire(c, notFound) })

			got, err := tt.call(ctx, api)

			var nf *survey.NotFoundError
			require.ErrorAs(t, err, &nf, "expected the domain error to be mapped, got %T", err)
			assert.Equal(t, "404", nf.Code)
			assert.Equal(t, "itx says missing", nf.Message)
			assert.Nil(t, got)
		})
	}
}
