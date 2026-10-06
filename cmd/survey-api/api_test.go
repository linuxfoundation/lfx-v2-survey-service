// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-survey-service/gen/survey"
	"github.com/linuxfoundation/lfx-v2-survey-service/internal/domain"
)

func TestMapDomainError_NonDomainError(t *testing.T) {
	err := mapDomainError(errors.New("plain error"))
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
			got := mapDomainError(domErr)

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
