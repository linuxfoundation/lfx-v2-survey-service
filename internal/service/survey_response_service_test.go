// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-survey-service/pkg/models/itx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	var domErr *domain.DomainError
	if !errors.As(err, &domErr) || domErr.Type != domain.ErrorTypeNotFound {
		t.Errorf("expected *domain.DomainError with ErrorTypeNotFound, got %T: %v", err, err)
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
	var domErr *domain.DomainError
	if !errors.As(err, &domErr) || domErr.Type != domain.ErrorTypeValidation {
		t.Fatalf("expected *domain.DomainError with ErrorTypeValidation, got %T: %v", err, err)
	}
	// Proxy must not have been called — the guard fires before any I/O.
	if proxy.CapturedParams != nil {
		t.Error("expected proxy not to be called when validation fails, but capturedParams is set")
	}
}

func newResponseService(sc *mocks.MockSurveyClient, rc *mocks.MockSurveyResponseClient, mapper *mocks.MockIDMapper) *service.SurveyService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return service.NewSurveyService(sc, &mocks.MockExclusionClient{}, rc, mapper, logger)
}

func TestDeleteSurveyResponse(t *testing.T) {
	t.Run("forwards the survey and response ids", func(t *testing.T) {
		var gotSurvey, gotResponse string
		svc := newResponseService(&mocks.MockSurveyClient{}, &mocks.MockSurveyResponseClient{
			DeleteResponseFunc: func(_ context.Context, surveyID, responseID string) error {
				gotSurvey, gotResponse = surveyID, responseID
				return nil
			},
		}, prefixMapper("", ""))

		require.NoError(t, svc.DeleteSurveyResponse(ctxWithPrincipal("alice"), &survey.DeleteSurveyResponsePayload{SurveyUID: "s-1", ResponseID: "r-1"}))
		assert.Equal(t, "s-1", gotSurvey)
		assert.Equal(t, "r-1", gotResponse)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewNotFoundError("no such response")
		svc := newResponseService(&mocks.MockSurveyClient{}, &mocks.MockSurveyResponseClient{
			DeleteResponseFunc: func(context.Context, string, string) error { return itxErr },
		}, prefixMapper("", ""))

		assert.Same(t, itxErr, svc.DeleteSurveyResponse(ctxWithPrincipal("alice"), &survey.DeleteSurveyResponsePayload{SurveyUID: "s-1", ResponseID: "r-1"}))
	})
}

func TestResendSurveyResponse(t *testing.T) {
	t.Run("forwards the survey and response ids", func(t *testing.T) {
		var gotSurvey, gotResponse string
		svc := newResponseService(&mocks.MockSurveyClient{}, &mocks.MockSurveyResponseClient{
			ResendResponseFunc: func(_ context.Context, surveyID, responseID string) error {
				gotSurvey, gotResponse = surveyID, responseID
				return nil
			},
		}, prefixMapper("", ""))

		require.NoError(t, svc.ResendSurveyResponse(ctxWithPrincipal("alice"), &survey.ResendSurveyResponsePayload{SurveyUID: "s-1", ResponseID: "r-1"}))
		assert.Equal(t, "s-1", gotSurvey)
		assert.Equal(t, "r-1", gotResponse)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewUnavailableError("itx down")
		svc := newResponseService(&mocks.MockSurveyClient{}, &mocks.MockSurveyResponseClient{
			ResendResponseFunc: func(context.Context, string, string) error { return itxErr },
		}, prefixMapper("", ""))

		assert.Same(t, itxErr, svc.ResendSurveyResponse(ctxWithPrincipal("alice"), &survey.ResendSurveyResponsePayload{SurveyUID: "s-1", ResponseID: "r-1"}))
	})
}

func TestDeleteRecipientGroup(t *testing.T) {
	var gotSurvey string
	var gotCommittee, gotProject, gotFoundation *string
	called := false
	client := &mocks.MockSurveyClient{
		DeleteRecipientGroupFunc: func(_ context.Context, surveyID string, committeeID, projectID, foundationID *string) error {
			called = true
			gotSurvey, gotCommittee, gotProject, gotFoundation = surveyID, committeeID, projectID, foundationID
			return nil
		},
	}
	svc := newResponseService(client, &mocks.MockSurveyResponseClient{}, prefixMapper("c-bad", ""))
	del := func(p *survey.DeleteRecipientGroupPayload) error {
		called = false
		return svc.DeleteRecipientGroup(ctxWithPrincipal("alice"), p)
	}

	t.Run("maps committee and project v2 to v1 and forwards the foundation id as is", func(t *testing.T) {
		require.NoError(t, del(&survey.DeleteRecipientGroupPayload{
			SurveyUID: "s-1", CommitteeUID: strPtr("c-uid"), ProjectUID: strPtr("p-uid"), FoundationID: strPtr("f-1"),
		}))
		assert.Equal(t, "s-1", gotSurvey)
		assert.Equal(t, strPtr("v1-committee-c-uid"), gotCommittee)
		assert.Equal(t, strPtr("v1-project-p-uid"), gotProject)
		assert.Equal(t, strPtr("f-1"), gotFoundation)
	})

	t.Run("omitted or empty ids are sent as nil", func(t *testing.T) {
		require.NoError(t, del(&survey.DeleteRecipientGroupPayload{SurveyUID: "s-1", CommitteeUID: strPtr(""), ProjectUID: nil}))
		assert.Nil(t, gotCommittee)
		assert.Nil(t, gotProject)
		assert.Nil(t, gotFoundation)
	})

	t.Run("committee mapping failure is returned and ITX is not called", func(t *testing.T) {
		err := del(&survey.DeleteRecipientGroupPayload{SurveyUID: "s-1", CommitteeUID: strPtr("c-bad")})
		assert.ErrorIs(t, err, errMapper)
		assert.False(t, called)
	})

	t.Run("project mapping failure is returned and ITX is not called", func(t *testing.T) {
		failing := newResponseService(client, &mocks.MockSurveyResponseClient{}, prefixMapper("p-bad", ""))
		called = false
		err := failing.DeleteRecipientGroup(ctxWithPrincipal("alice"), &survey.DeleteRecipientGroupPayload{SurveyUID: "s-1", ProjectUID: strPtr("p-bad")})
		assert.ErrorIs(t, err, errMapper)
		assert.False(t, called)
	})

	t.Run("ITX error is returned unchanged", func(t *testing.T) {
		itxErr := domain.NewNotFoundError("no such group")
		failing := newResponseService(&mocks.MockSurveyClient{
			DeleteRecipientGroupFunc: func(context.Context, string, *string, *string, *string) error { return itxErr },
		}, &mocks.MockSurveyResponseClient{}, prefixMapper("", ""))

		assert.Same(t, itxErr, failing.DeleteRecipientGroup(ctxWithPrincipal("alice"), &survey.DeleteRecipientGroupPayload{SurveyUID: "s-1"}))
	})
}

func TestListSurveyResponses_MapsEveryField(t *testing.T) {
	rc := &mocks.MockSurveyResponseClient{
		ListResponsesResult: &itx.PaginatedSurveyResponses{
			Data: []itx.SurveyRecipientResponse{{
				ID: "r-1", SurveyID: "s-1", SurveyLink: strPtr("https://example.com/r-1"),
				CommitteeID: strPtr("c1"), Email: strPtr("alice@example.com"), FirstName: strPtr("Alice"), LastName: strPtr("Example"),
				Username: strPtr("alice"), Role: strPtr("Chair"), JobTitle: strPtr("Engineer"), MembershipTier: strPtr("Gold"), VotingStatus: strPtr("Voting Rep"),
				Organization:   &itx.SurveyResponseOrganization{ID: strPtr("org-1"), Name: strPtr("Test Org")},
				Project:        &itx.SurveyResponseProject{ID: strPtr("p1"), Name: strPtr("Project One")},
				ResponseStatus: strPtr("Responded"), CreatedAt: strPtr("2026-01-01T00:00:00Z"), ResponseDatetime: strPtr("2026-01-02T00:00:00Z"),
				LastReceivedTime: strPtr("2026-01-03T00:00:00Z"), NumAutomatedRemindersReceived: intPtr(2), NPSValue: f64Ptr(9),
				SurveyMonkeyRespondentID: strPtr("resp-1"),
				SurveyMonkeyQuestionAnswers: []itx.SurveyMonkeyQuestionAnswer{
					{QuestionID: "q-1", QuestionText: strPtr("Rate us"), QuestionFamily: strPtr("single_choice"), QuestionSubtype: strPtr("vertical"),
						Answers: []itx.SurveyMonkeyAnswer{{ChoiceID: strPtr("c-1"), Text: strPtr("10")}, {ChoiceID: strPtr("c-2"), Text: strPtr("9")}}},
					{QuestionID: "q-2"},
				},
				SESMessageID: strPtr("ses-1"), SESDeliverySuccessful: boolPtr(true), SESBounceType: strPtr("Permanent"), SESBounceSubtype: strPtr("General"),
				SESBounceDiagnosticCode: strPtr("550"), SESComplaintExists: boolPtr(true), SESComplaintType: strPtr("abuse"), SESComplaintDate: strPtr("2026-01-04T00:00:00Z"),
				SESEmailOpened: boolPtr(true), SESEmailOpenedLastTime: strPtr("2026-01-05T00:00:00Z"), SESLinkClicked: boolPtr(true), SESLinkClickedLastTime: strPtr("2026-01-06T00:00:00Z"),
			}},
			Meta: itx.PageMetadata{PageToken: "next", TotalPages: 3, TotalResults: 30, PerPage: 10},
		},
	}
	svc := newResponseService(&mocks.MockSurveyClient{}, rc, prefixMapper("", ""))

	got, err := svc.ListSurveyResponses(ctxWithPrincipal("alice"), &survey.ListSurveyResponsesPayload{SurveyUID: "s-1"})

	require.NoError(t, err)
	require.Len(t, got.Data, 1)
	assert.Equal(t, &survey.SurveyResponseItem{
		ID: "r-1", SurveyUID: "s-1", SurveyLink: strPtr("https://example.com/r-1"),
		CommitteeUID: strPtr("v2-committee-c1"), Email: strPtr("alice@example.com"), FirstName: strPtr("Alice"), LastName: strPtr("Example"),
		Username: strPtr("alice"), Role: strPtr("Chair"), JobTitle: strPtr("Engineer"), MembershipTier: strPtr("Gold"), VotingStatus: strPtr("Voting Rep"),
		Organization:   &survey.SurveyResponseOrg{ID: strPtr("org-1"), Name: strPtr("Test Org")},
		Project:        &survey.SurveyResponseProj{UID: strPtr("v2-project-p1"), Name: strPtr("Project One")},
		ResponseStatus: strPtr("Responded"), CreatedAt: strPtr("2026-01-01T00:00:00Z"), ResponseDatetime: strPtr("2026-01-02T00:00:00Z"),
		LastReceivedTime: strPtr("2026-01-03T00:00:00Z"), NumAutomatedRemindersReceived: intPtr(2), NpsValue: f64Ptr(9),
		SurveyMonkeyRespondentID: strPtr("resp-1"),
		SurveyMonkeyQuestionAnswers: []*survey.SurveyQuestionAnswer{
			{QuestionID: "q-1", QuestionText: strPtr("Rate us"), QuestionFamily: strPtr("single_choice"), QuestionSubtype: strPtr("vertical"),
				Answers: []*survey.SurveyAnswerChoice{{ChoiceID: strPtr("c-1"), Text: strPtr("10")}, {ChoiceID: strPtr("c-2"), Text: strPtr("9")}}},
			{QuestionID: "q-2", Answers: []*survey.SurveyAnswerChoice{}},
		},
		SesMessageID: strPtr("ses-1"), SesDeliverySuccessful: boolPtr(true), SesBounceType: strPtr("Permanent"), SesBounceSubtype: strPtr("General"),
		SesBounceDiagnosticCode: strPtr("550"), SesComplaintExists: boolPtr(true), SesComplaintType: strPtr("abuse"), SesComplaintDate: strPtr("2026-01-04T00:00:00Z"),
		SesEmailOpened: boolPtr(true), SesEmailOpenedLastTime: strPtr("2026-01-05T00:00:00Z"), SesLinkClicked: boolPtr(true), SesLinkClickedLastTime: strPtr("2026-01-06T00:00:00Z"),
	}, got.Data[0])
	assert.Equal(t, &survey.SurveyResponsePageMeta{PageToken: strPtr("next"), TotalPages: intPtr(3), TotalResults: intPtr(30), PerPage: intPtr(10)}, got.Meta)
}

func TestListSurveyResponses_IDMapping(t *testing.T) {
	respond := func(r itx.SurveyRecipientResponse) *mocks.MockSurveyResponseClient {
		return &mocks.MockSurveyResponseClient{ListResponsesResult: &itx.PaginatedSurveyResponses{Data: []itx.SurveyRecipientResponse{r}}}
	}

	t.Run("committee and project keep their v1 ids when mapping fails", func(t *testing.T) {
		svc := newResponseService(&mocks.MockSurveyClient{}, respond(itx.SurveyRecipientResponse{
			ID: "r-1", CommitteeID: strPtr("c-bad"), Project: &itx.SurveyResponseProject{ID: strPtr("p-bad")},
		}), &mocks.MockIDMapper{
			MapCommitteeV1ToV2Func: func(context.Context, string) (string, error) { return "", errMapper },
			MapProjectV1ToV2Func:   func(context.Context, string) (string, error) { return "", errMapper },
		})

		got, err := svc.ListSurveyResponses(ctxWithPrincipal("alice"), &survey.ListSurveyResponsesPayload{SurveyUID: "s-1"})

		require.NoError(t, err)
		assert.Equal(t, "c-bad", *got.Data[0].CommitteeUID)
		assert.Equal(t, "p-bad", *got.Data[0].Project.UID)
	})

	t.Run("no committee, project, organization or answers", func(t *testing.T) {
		svc := newResponseService(&mocks.MockSurveyClient{}, respond(itx.SurveyRecipientResponse{ID: "r-1"}), prefixMapper("", ""))

		got, err := svc.ListSurveyResponses(ctxWithPrincipal("alice"), &survey.ListSurveyResponsesPayload{SurveyUID: "s-1"})

		require.NoError(t, err)
		item := got.Data[0]
		assert.Nil(t, item.CommitteeUID)
		assert.Nil(t, item.Project)
		assert.Nil(t, item.Organization)
		assert.NotNil(t, item.SurveyMonkeyQuestionAnswers, "answers are always an empty list, never null")
		assert.Empty(t, item.SurveyMonkeyQuestionAnswers)
	})

	t.Run("project_uid mapping failure is returned and ITX is not called", func(t *testing.T) {
		rc := &mocks.MockSurveyResponseClient{}
		svc := newResponseService(&mocks.MockSurveyClient{}, rc, prefixMapper("p-bad", ""))

		_, err := svc.ListSurveyResponses(ctxWithPrincipal("alice"), &survey.ListSurveyResponsesPayload{SurveyUID: "s-1", ProjectUID: strPtr("p-bad")})

		assert.ErrorIs(t, err, errMapper)
		assert.Empty(t, rc.CapturedSurveyID)
	})

	t.Run("project_uids mapping failure is returned and ITX is not called", func(t *testing.T) {
		rc := &mocks.MockSurveyResponseClient{}
		svc := newResponseService(&mocks.MockSurveyClient{}, rc, prefixMapper("p-bad", ""))

		_, err := svc.ListSurveyResponses(ctxWithPrincipal("alice"), &survey.ListSurveyResponsesPayload{SurveyUID: "s-1", ProjectUids: strPtr("p1,p-bad")})

		assert.ErrorIs(t, err, errMapper)
		assert.Empty(t, rc.CapturedSurveyID)
	})

	t.Run("one failing response mapping does not drop the others", func(t *testing.T) {
		rc := &mocks.MockSurveyResponseClient{ListResponsesResult: &itx.PaginatedSurveyResponses{Data: []itx.SurveyRecipientResponse{
			{ID: "r-1", CommitteeID: strPtr("c1")}, {ID: "r-2", CommitteeID: strPtr("c-bad")}, {ID: "r-3", CommitteeID: strPtr("c3")},
		}}}
		svc := newResponseService(&mocks.MockSurveyClient{}, rc, prefixMapper("", "c-bad"))

		got, err := svc.ListSurveyResponses(ctxWithPrincipal("alice"), &survey.ListSurveyResponsesPayload{SurveyUID: "s-1"})

		require.NoError(t, err)
		require.Len(t, got.Data, 3)
		assert.Equal(t, "v2-committee-c1", *got.Data[0].CommitteeUID)
		assert.Equal(t, "c-bad", *got.Data[1].CommitteeUID)
		assert.Equal(t, "v2-committee-c3", *got.Data[2].CommitteeUID)
	})
}
