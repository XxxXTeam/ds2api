package client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/auth"
)

func TestExtractCreateSessionIDSupportsLegacyShape(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"biz_data": map[string]any{
				"id": "legacy-session-id",
			},
		},
	}

	if got := extractCreateSessionID(resp); got != "legacy-session-id" {
		t.Fatalf("expected legacy session id, got %q", got)
	}
}

func TestExtractCreateSessionIDSupportsNestedChatSessionShape(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"biz_data": map[string]any{
				"chat_session": map[string]any{
					"id":         "nested-session-id",
					"model_type": "default",
				},
			},
		},
	}

	if got := extractCreateSessionID(resp); got != "nested-session-id" {
		t.Fatalf("expected nested session id, got %q", got)
	}
}

func TestCreateSessionSendsEmptyBody(t *testing.T) {
	var gotBody string
	client := &Client{
		regular: doerFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll request body: %v", err)
			}
			gotBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"code": 0,
					"data": {
						"biz_code": 0,
						"biz_data": {
							"id": "session-id"
						}
					}
				}`)),
				Request: req,
			}, nil
		}),
	}

	sessionID, err := client.CreateSession(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, 1)
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	if sessionID != "session-id" {
		t.Fatalf("sessionID=%q want session-id", sessionID)
	}
	if gotBody != "" {
		t.Fatalf("expected empty create-session body, got %q", gotBody)
	}
}
