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
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/infrastructure/idmapper"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
)

// mockAuth is a test double for domain.Authenticator
type mockAuth struct {
	principal string
	err       error
}

func (m *mockAuth) ParsePrincipal(_ context.Context, _ string, _ *slog.Logger) (string, error) {
	return m.principal, m.err
}

// mockProxy is a test double for domain.ITXProxyClient
// Only ListResponses is implemented; all others panic if called unexpectedly.
type mockProxy struct {
	listResponsesResult *itx.PaginatedSurveyResponses
	listResponsesErr    error
	capturedSurveyID    string
	capturedParams      *itx.ListResponsesParams
}

func (m *mockProxy) ScheduleSurvey(_ context.Context, _ *itx.ScheduleSurveyRequest) (*itx.SurveyScheduleResponse, error) {
	panic("unexpected call to ScheduleSurvey")
}
func (m *mockProxy) GetSurvey(_ context.Context, _ string, _ *itx.GetSurveyParams) (*itx.SurveyScheduleResponse, error) {
	panic("unexpected call to GetSurvey")
}
func (m *mockProxy) UpdateSurvey(_ context.Context, _ string, _ *itx.UpdateSurveyRequest) (*itx.SurveyScheduleResponse, error) {
	panic("unexpected call to UpdateSurvey")
}
func (m *mockProxy) DeleteSurvey(_ context.Context, _ string) error {
	panic("unexpected call to DeleteSurvey")
}
func (m *mockProxy) ExtendSurvey(_ context.Context, _ string, _ *itx.ExtendSurveyRequest) (*itx.SurveyScheduleResponse, error) {
	panic("unexpected call to ExtendSurvey")
}
func (m *mockProxy) EnableSurvey(_ context.Context, _ string) error {
	panic("unexpected call to EnableSurvey")
}
func (m *mockProxy) BulkResendSurvey(_ context.Context, _ string, _ *itx.BulkResendRequest) error {
	panic("unexpected call to BulkResendSurvey")
}
func (m *mockProxy) PreviewSend(_ context.Context, _ string, _ *string) (*itx.PreviewSendResponse, error) {
	panic("unexpected call to PreviewSend")
}
func (m *mockProxy) SendMissingRecipients(_ context.Context, _ string, _ *string) error {
	panic("unexpected call to SendMissingRecipients")
}
func (m *mockProxy) DeleteRecipientGroup(_ context.Context, _ string, _ *string, _ *string, _ *string) error {
	panic("unexpected call to DeleteRecipientGroup")
}
func (m *mockProxy) CreateExclusion(_ context.Context, _ *itx.ExclusionRequest) (*itx.Exclusion, error) {
	panic("unexpected call to CreateExclusion")
}
func (m *mockProxy) DeleteExclusion(_ context.Context, _ *itx.ExclusionRequest) error {
	panic("unexpected call to DeleteExclusion")
}
func (m *mockProxy) GetExclusion(_ context.Context, _ string) (*itx.ExtendedExclusion, error) {
	panic("unexpected call to GetExclusion")
}
func (m *mockProxy) DeleteExclusionByID(_ context.Context, _ string) error {
	panic("unexpected call to DeleteExclusionByID")
}
func (m *mockProxy) GetSurveyResults(_ context.Context, _ string) (*itx.SurveyResults, error) {
	panic("unexpected call to GetSurveyResults")
}
func (m *mockProxy) ValidateEmail(_ context.Context, _ *itx.ValidateEmailRequest) (*itx.ValidateEmailResponse, error) {
	panic("unexpected call to ValidateEmail")
}
func (m *mockProxy) CreateResponse(_ context.Context, _ *itx.CreateResponseRequest) error {
	panic("unexpected call to CreateResponse")
}
func (m *mockProxy) GetResponse(_ context.Context, _ string) (*itx.ResponseResponse, error) {
	panic("unexpected call to GetResponse")
}
func (m *mockProxy) UpdateResponse(_ context.Context, _ string, _ *itx.UpdateResponseRequest) error {
	panic("unexpected call to UpdateResponse")
}
func (m *mockProxy) DeleteResponse(_ context.Context, _ string, _ string) error {
	panic("unexpected call to DeleteResponse")
}
func (m *mockProxy) ResendResponse(_ context.Context, _ string, _ string) error {
	panic("unexpected call to ResendResponse")
}
func (m *mockProxy) ListResponses(_ context.Context, surveyID string, params *itx.ListResponsesParams) (*itx.PaginatedSurveyResponses, error) {
	m.capturedSurveyID = surveyID
	m.capturedParams = params
	return m.listResponsesResult, m.listResponsesErr
}

// helpers

func strPtr(s string) *string   { return &s }
func f64Ptr(f float64) *float64 { return &f }

func newTestService(proxy domain.ITXProxyClient) *service.SurveyService {
	auth := &mockAuth{principal: "test-user"}
	mapper := idmapper.NewNoOpMapper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return service.NewSurveyService(auth, proxy, mapper, logger)
}

func TestListSurveyResponses_Success(t *testing.T) {
	questionID := "q-001"
	questionText := "How satisfied are you?"
	choiceID := "c-001"
	choiceText := "Very satisfied"

	proxy := &mockProxy{
		listResponsesResult: &itx.PaginatedSurveyResponses{
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
	proxy := &mockProxy{
		listResponsesResult: &itx.PaginatedSurveyResponses{
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
	proxy := &mockProxy{
		listResponsesResult: &itx.PaginatedSurveyResponses{
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
	proxy := &mockProxy{
		listResponsesErr: domain.NewNotFoundError("survey not found", nil),
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
	proxy := &mockProxy{
		listResponsesResult: &itx.PaginatedSurveyResponses{
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
	if proxy.capturedParams == nil {
		t.Fatal("expected params to be forwarded, got nil")
	}
	// NoOpMapper returns each V2 UID unchanged — joined result should equal input
	if proxy.capturedParams.ProjectIDs == nil || *proxy.capturedParams.ProjectIDs != "uid-one,uid-two,uid-three" {
		t.Errorf("expected ProjectIDs uid-one,uid-two,uid-three, got %v", proxy.capturedParams.ProjectIDs)
	}
	// ProjectID should be nil when only project_uids was provided
	if proxy.capturedParams.ProjectID != nil {
		t.Errorf("expected ProjectID nil when only project_uids provided, got %v", proxy.capturedParams.ProjectID)
	}
}

func TestListSurveyResponses_ProjectUID_ForwardedToProxy(t *testing.T) {
	// Verify project_uid is V2→V1 mapped and forwarded as ProjectID in ListResponses params.
	// NoOpMapper returns the ID unchanged, so we can assert the value directly.
	proxy := &mockProxy{
		listResponsesResult: &itx.PaginatedSurveyResponses{
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
	if proxy.capturedSurveyID != "survey-uid-abc" {
		t.Errorf("expected survey_uid survey-uid-abc forwarded, got %q", proxy.capturedSurveyID)
	}
	if proxy.capturedParams == nil {
		t.Fatal("expected params to be forwarded, got nil")
	}
	// NoOpMapper returns V2 UID unchanged — assert it was set as ProjectID
	if proxy.capturedParams.ProjectID == nil || *proxy.capturedParams.ProjectID != "v2-project-uid" {
		t.Errorf("expected ProjectID v2-project-uid forwarded, got %v", proxy.capturedParams.ProjectID)
	}
	// Pagination params passed through unchanged
	if proxy.capturedParams.PageToken == nil || *proxy.capturedParams.PageToken != "tok-abc" {
		t.Errorf("expected PageToken tok-abc forwarded, got %v", proxy.capturedParams.PageToken)
	}
	if proxy.capturedParams.PerPage == nil || *proxy.capturedParams.PerPage != "10" {
		t.Errorf("expected PerPage 10 forwarded, got %v", proxy.capturedParams.PerPage)
	}
}

func TestListSurveyResponses_BothProjectFilters_ReturnsValidationError(t *testing.T) {
	// project_uid and project_uids are mutually exclusive. Providing both must be rejected
	// with a 400 Bad Request before any proxy or ID-mapping calls are made.
	proxy := &mockProxy{}
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
	if proxy.capturedParams != nil {
		t.Error("expected proxy not to be called when validation fails, but capturedParams is set")
	}
}
