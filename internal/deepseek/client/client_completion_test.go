package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/auth"
)

func TestCallCompletionDoesNotFallbackForNonIdempotentCompletion(t *testing.T) {
	var fallbackCalled bool
	client := &Client{
		stream: doerFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("ambiguous completion write failure")
		}),
		fallbackS: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			fallbackCalled = true
			return &http.Response{StatusCode: http.StatusOK}, nil
		})},
	}
	_, err := client.CallCompletion(
		context.Background(),
		&auth.RequestAuth{DeepSeekToken: "token"},
		map[string]any{"prompt": "hello"},
		"pow",
		3,
	)
	if err == nil {
		t.Fatal("expected completion error")
	}
	if fallbackCalled {
		t.Fatal("completion fallback should not be called for a non-idempotent request")
	}
}

func TestStreamPostDecodesGzipResponse(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte("data: hello\n\n")); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	client := &Client{}
	resp, err := client.streamPostOnce(
		context.Background(),
		doerFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Encoding": []string{"gzip"},
					"Content-Length":   []string{"999"},
				},
				Body:    io.NopCloser(bytes.NewReader(buf.Bytes())),
				Request: req,
			}, nil
		}),
		"https://example.com/stream",
		map[string]string{},
		map[string]any{"prompt": "hello"},
	)
	if err != nil {
		t.Fatalf("streamPostOnce returned error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll response body: %v", err)
	}
	if got := string(body); got != "data: hello\n\n" {
		t.Fatalf("decoded body=%q", got)
	}
	if got := strings.TrimSpace(resp.Header.Get("Content-Encoding")); got != "" {
		t.Fatalf("Content-Encoding should be cleared, got %q", got)
	}
	if got := strings.TrimSpace(resp.Header.Get("Content-Length")); got != "" {
		t.Fatalf("Content-Length should be cleared, got %q", got)
	}
}
