// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
)

// ─── ListSurveyTemplates ─────────────────────────────────────────────────────

func TestListSurveyTemplates_GetsTemplatesPath(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{"id":"412349483","title":"Board Member NPS","nickname":"Board NPS","href":"https://api.surveymonkey.com/v3/surveys/412349483"},
			{"id":"512349484","title":"Contributor NPS","nickname":"","href":"https://api.surveymonkey.com/v3/surveys/512349484"}
		]`))
	}))
	defer srv.Close()

	templates, err := testClient(srv).ListSurveyTemplates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("expected method GET, got %q", gotMethod)
	}
	if gotPath != "/v2/surveys/templates" {
		t.Errorf("expected path /v2/surveys/templates, got %q", gotPath)
	}
	if len(templates) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(templates))
	}
	first := templates[0]
	if first.ID != "412349483" || first.Title != "Board Member NPS" || first.Nickname != "Board NPS" ||
		first.Href != "https://api.surveymonkey.com/v3/surveys/412349483" {
		t.Errorf("unexpected first template: %+v", first)
	}
	if templates[1].ID != "512349484" || templates[1].Nickname != "" {
		t.Errorf("unexpected second template: %+v", templates[1])
	}
}

func TestListSurveyTemplates_NullBodyReturnsEmptySlice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`null`))
	}))
	defer srv.Close()

	templates, err := testClient(srv).ListSurveyTemplates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if templates == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(templates) != 0 {
		t.Errorf("expected empty slice, got %d items", len(templates))
	}
}

func TestListSurveyTemplates_Non2xxMapsToDomainError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"403", "message":"not authorized for this method"}`))
	}))
	defer srv.Close()

	templates, err := testClient(srv).ListSurveyTemplates(context.Background())
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if templates != nil {
		t.Errorf("expected nil templates on error, got %+v", templates)
	}
	var domErr *domain.DomainError
	if !errors.As(err, &domErr) {
		t.Fatalf("expected *domain.DomainError, got %T: %v", err, err)
	}
}
