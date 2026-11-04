// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"text/template"

	surveysvr "github.com/linuxfoundation/lfx-v2-survey-service/gen/http/survey/server"
	surveysvc "github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/infrastructure/idmapper"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
	goahttp "goa.design/goa/v3/http"
	"gopkg.in/yaml.v3"
)

// newRoutingTestHandler mounts the generated survey HTTP server the same way
// main.go does, backed by the given survey client mock.
func newRoutingTestHandler(client *mocks.MockSurveyClient) http.Handler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	surveyService := service.NewSurveyService(client, &mocks.MockExclusionClient{}, &mocks.MockSurveyResponseClient{}, idmapper.NewNoOpMapper(), logger)
	surveyAPI := NewSurveyAPI(surveyService, &mocks.MockAuthenticator{Principal: "test-user"})

	mux := goahttp.NewMuxer()
	surveyServer := surveysvr.New(surveysvc.NewEndpoints(surveyAPI), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
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
	if values["relation"] != "writer" {
		t.Fatalf("expected primary relation writer, got %q", values["relation"])
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
