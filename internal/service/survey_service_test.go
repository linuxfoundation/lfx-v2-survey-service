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
func (m *mockProxy) ListResponses(_ context.Context, _ string, _ *itx.ListResponsesParams) (*itx.PaginatedSurveyResponses, error) {
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

func TestListSurveyResponses_ProjectUID_MappedToV1(t *testing.T) {
	// Verify that providing project_uid doesn't error (noop mapper returns it unchanged, proxy succeeds)
	proxy := &mockProxy{
		listResponsesResult: &itx.PaginatedSurveyResponses{
			Data: []itx.SurveyRecipientResponse{},
			Meta: itx.PageMetadata{},
		},
	}

	svc := newTestService(proxy)
	token := "test-token"
	projectUID := "v2-project-uid"

	_, err := svc.ListSurveyResponses(context.Background(), &survey.ListSurveyResponsesPayload{
		Token:      &token,
		SurveyUID:  "survey-uid-abc",
		ProjectUID: &projectUID,
	})

	if err != nil {
		t.Fatalf("unexpected error when project_uid provided: %v", err)
	}
}
