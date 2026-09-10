// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/infrastructure/idmapper"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

// Test doubles for domain interfaces are provided by internal/domain/mocks.
// Do not re-declare mock types here — add to or extend the shared package instead.

// helpers

func strPtr(s string) *string   { return &s }
func f64Ptr(f float64) *float64 { return &f }

func newTestService(responseClient *mocks.MockSurveyResponseClient) *service.SurveyService {
	mapper := idmapper.NewNoOpMapper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return service.NewSurveyService(&mocks.MockSurveyClient{}, &mocks.MockExclusionClient{}, responseClient, mapper, logger)
}

func TestListSurveyResponses_Success(t *testing.T) {
	questionID := "q-001"
	questionText := "How satisfied are you?"
	choiceID := "c-001"
	choiceText := "Very satisfied"

	proxy := &mocks.MockSurveyResponseClient{
		ListResponsesResult: &itx.PaginatedSurveyResponses{
			Data: []itx.SurveyRecipientResponse{
				{
					ID:             "resp-001",
					SurveyID:       "survey-uid-abc",
					Email:          strPtr("jane.doe@example.com"),
					FirstName:      strPtr("Jane"),
					LastName:       strPtr("Doe"),
					ResponseStatus: strPtr("Responded"),
					NPSValue:       f64Ptr(9.0),
					Project:        &itx.SurveyResponseProject{ID: strPtr("proj-v1"), Name: strPtr("Kubernetes")},
					Organization:   &itx.SurveyResponseOrganization{ID: strPtr("org-001"), Name: strPtr("Acme Corp")},
					SurveyMonkeyQuestionAnswers: []itx.SurveyMonkeyQuestionAnswer{
						{
							QuestionID:   questionID,
							QuestionText: &questionText,
							Answers:      []itx.SurveyMonkeyAnswer{{ChoiceID: &choiceID, Text: &choiceText}},
						},
					},
				},
			},
			Meta: itx.PageMetadata{
				PageToken:    "",
				TotalPages:   1,
				TotalResults: 1,
				PerPage:      25,
			},
		},
	}

	svc := newTestService(proxy)
	token := "test-token"
	surveyUID := "survey-uid-abc"

	result, err := svc.ListSurveyResponses(context.Background(), &survey.ListSurveyResponsesPayload{
		Token:     &token,
		SurveyUID: surveyUID,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 response, got %d", len(result.Data))
	}

	item := result.Data[0]
	if item.ID != "resp-001" {
		t.Errorf("expected ID resp-001, got %s", item.ID)
	}
	if item.Email == nil || *item.Email != "jane.doe@example.com" {
		t.Errorf("expected email jane.doe@example.com, got %v", item.Email)
	}
	if len(item.SurveyMonkeyQuestionAnswers) != 1 {
		t.Fatalf("expected 1 question answer, got %d", len(item.SurveyMonkeyQuestionAnswers))
	}
	qa := item.SurveyMonkeyQuestionAnswers[0]
	if qa.QuestionID != "q-001" {
		t.Errorf("expected question_id q-001, got %s", qa.QuestionID)
	}
	if len(qa.Answers) != 1 || qa.Answers[0].Text == nil || *qa.Answers[0].Text != "Very satisfied" {
		t.Errorf("unexpected answers: %+v", qa.Answers)
	}
	if item.Project == nil || item.Project.Name == nil || *item.Project.Name != "Kubernetes" {
		t.Errorf("expected project name Kubernetes, got %v", item.Project)
	}
	// NoOpMapper returns V1 ID unchanged, so UID should equal the V1 input
	if item.Project.UID == nil || *item.Project.UID != "proj-v1" {
		t.Errorf("expected project UID proj-v1 (noop mapper), got %v", item.Project.UID)
	}

	// Verify pagination meta
	if result.Meta == nil {
		t.Fatal("expected meta, got nil")
	}
	if result.Meta.TotalResults == nil || *result.Meta.TotalResults != 1 {
		t.Errorf("expected TotalResults 1, got %v", result.Meta.TotalResults)
	}
}

func TestListSurveyResponses_ProjectWithNoID_OmitsUID(t *testing.T) {
	// When a response has a project object but no id, UID must be nil (not a pointer to "").
	proxy := &mocks.MockSurveyResponseClient{
		ListResponsesResult: &itx.PaginatedSurveyResponses{
			Data: []itx.SurveyRecipientResponse{
				{
					ID:       "resp-002",
					SurveyID: "survey-uid-abc",
					Project:  &itx.SurveyResponseProject{ID: nil, Name: strPtr("Unnamed Project")},
				},
			},
			Meta: itx.PageMetadata{},
		},
	}

	svc := newTestService(proxy)
	token := "test-token"

	result, err := svc.ListSurveyResponses(context.Background(), &survey.ListSurveyResponsesPayload{
		Token:     &token,
		SurveyUID: "survey-uid-abc",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	item := result.Data[0]
	if item.Project == nil {
		t.Fatal("expected project to be present")
	}
	if item.Project.UID != nil {
		t.Errorf("expected project UID to be nil when no id provided, got %q", *item.Project.UID)
	}
}

func TestListSurveyResponses_EmptyData(t *testing.T) {
	proxy := &mocks.MockSurveyResponseClient{
		ListResponsesResult: &itx.PaginatedSurveyResponses{
			Data: []itx.SurveyRecipientResponse{},
			Meta: itx.PageMetadata{TotalResults: 0, TotalPages: 0, PerPage: 25},
		},
	}

	svc := newTestService(proxy)
	token := "test-token"

	result, err := svc.ListSurveyResponses(context.Background(), &survey.ListSurveyResponsesPayload{
		Token:     &token,
		SurveyUID: "survey-uid-empty",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if len(result.Data) != 0 {
		t.Errorf("expected empty data slice, got %d items", len(result.Data))
	}
}

func TestListSurveyResponses_ITX404_MapsToNotFound(t *testing.T) {
	proxy := &mocks.MockSurveyResponseClient{
		ListResponsesErr: domain.NewNotFoundError("survey not found", nil),
	}

	svc := newTestService(proxy)
	token := "test-token"

	_, err := svc.ListSurveyResponses(context.Background(), &survey.ListSurveyResponsesPayload{
		Token:     &token,
		SurveyUID: "nonexistent-survey",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := err.(*survey.NotFoundError); !ok {
		t.Errorf("expected *survey.NotFoundError, got %T: %v", err, err)
	}
}

func TestListSurveyResponses_ProjectUIDs_ForwardedToProxy(t *testing.T) {
	// Verify that comma-delimited project_uids is V2→V1 mapped and forwarded as ProjectIDs.
	// NoOpMapper returns each ID unchanged, so the joined string should equal the input.
	proxy := &mocks.MockSurveyResponseClient{
		ListResponsesResult: &itx.PaginatedSurveyResponses{
			Data: []itx.SurveyRecipientResponse{},
			Meta: itx.PageMetadata{},
		},
	}

	svc := newTestService(proxy)
	token := "test-token"
	projectUIDs := "uid-one,uid-two,uid-three"

	_, err := svc.ListSurveyResponses(context.Background(), &survey.ListSurveyResponsesPayload{
		Token:       &token,
		SurveyUID:   "survey-uid-abc",
		ProjectUids: &projectUIDs,
	})

	if err != nil {
		t.Fatalf("unexpected error when project_uids provided: %v", err)
	}
	if proxy.CapturedParams == nil {
		t.Fatal("expected params to be forwarded, got nil")
	}
	// NoOpMapper returns each V2 UID unchanged — joined result should equal input
	if proxy.CapturedParams.ProjectIDs == nil || *proxy.CapturedParams.ProjectIDs != "uid-one,uid-two,uid-three" {
		t.Errorf("expected ProjectIDs uid-one,uid-two,uid-three, got %v", proxy.CapturedParams.ProjectIDs)
	}
	// ProjectID should be nil when only project_uids was provided
	if proxy.CapturedParams.ProjectID != nil {
		t.Errorf("expected ProjectID nil when only project_uids provided, got %v", proxy.CapturedParams.ProjectID)
	}
}

func TestListSurveyResponses_ProjectUID_ForwardedToProxy(t *testing.T) {
	// Verify project_uid is V2→V1 mapped and forwarded as ProjectID in ListResponses params.
	// NoOpMapper returns the ID unchanged, so we can assert the value directly.
	proxy := &mocks.MockSurveyResponseClient{
		ListResponsesResult: &itx.PaginatedSurveyResponses{
			Data: []itx.SurveyRecipientResponse{},
			Meta: itx.PageMetadata{},
		},
	}

	svc := newTestService(proxy)
	token := "test-token"
	projectUID := "v2-project-uid"
	pageToken := "tok-abc"
	perPage := "10"

	_, err := svc.ListSurveyResponses(context.Background(), &survey.ListSurveyResponsesPayload{
		Token:      &token,
		SurveyUID:  "survey-uid-abc",
		ProjectUID: &projectUID,
		PageToken:  &pageToken,
		PerPage:    &perPage,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proxy.CapturedSurveyID != "survey-uid-abc" {
		t.Errorf("expected survey_uid survey-uid-abc forwarded, got %q", proxy.CapturedSurveyID)
	}
	if proxy.CapturedParams == nil {
		t.Fatal("expected params to be forwarded, got nil")
	}
	// NoOpMapper returns V2 UID unchanged — assert it was set as ProjectID
	if proxy.CapturedParams.ProjectID == nil || *proxy.CapturedParams.ProjectID != "v2-project-uid" {
		t.Errorf("expected ProjectID v2-project-uid forwarded, got %v", proxy.CapturedParams.ProjectID)
	}
	// Pagination params passed through unchanged
	if proxy.CapturedParams.PageToken == nil || *proxy.CapturedParams.PageToken != "tok-abc" {
		t.Errorf("expected PageToken tok-abc forwarded, got %v", proxy.CapturedParams.PageToken)
	}
	if proxy.CapturedParams.PerPage == nil || *proxy.CapturedParams.PerPage != "10" {
		t.Errorf("expected PerPage 10 forwarded, got %v", proxy.CapturedParams.PerPage)
	}
}

func TestListSurveyResponses_BothProjectFilters_ReturnsValidationError(t *testing.T) {
	// project_uid and project_uids are mutually exclusive. Providing both must be rejected
	// with a 400 Bad Request before any proxy or ID-mapping calls are made.
	proxy := &mocks.MockSurveyResponseClient{}
	svc := newTestService(proxy)
	token := "test-token"
	projectUID := "v2-project-uid"
	projectUIDs := "v2-uid-one,v2-uid-two"

	_, err := svc.ListSurveyResponses(context.Background(), &survey.ListSurveyResponsesPayload{
		Token:       &token,
		SurveyUID:   "survey-uid-abc",
		ProjectUID:  &projectUID,
		ProjectUids: &projectUIDs,
	})

	if err == nil {
		t.Fatal("expected a validation error when both project_uid and project_uids are set, got nil")
	}
	var badReq *survey.BadRequestError
	if !func() bool {
		e, ok := err.(*survey.BadRequestError)
		if ok {
			badReq = e
		}
		return ok
	}() {
		t.Fatalf("expected *survey.BadRequestError, got %T: %v", err, err)
	}
	if badReq.Code != "400" {
		t.Errorf("expected code 400, got %q", badReq.Code)
	}
	// Proxy must not have been called — the guard fires before any I/O.
	if proxy.CapturedParams != nil {
		t.Error("expected proxy not to be called when validation fails, but capturedParams is set")
	}
}
