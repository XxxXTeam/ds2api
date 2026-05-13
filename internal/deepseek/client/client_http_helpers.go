package client

import (
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"

	"ds2api/internal/config"
	dsprotocol "ds2api/internal/deepseek/protocol"
)

func readResponseBody(resp *http.Response) ([]byte, error) {
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	var reader io.Reader = resp.Body
	switch encoding {
	case "gzip":
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer func() { _ = gz.Close() }()
		reader = gz
	case "br":
		reader = brotli.NewReader(resp.Body)
	}
	return io.ReadAll(reader)
}

type decodedResponseBody struct {
	decoder io.Closer
	body    io.Closer
	reader  io.Reader
}

func (b decodedResponseBody) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

func (b decodedResponseBody) Close() error {
	return errors.Join(b.decoder.Close(), b.body.Close())
}

func decodeStreamingResponse(resp *http.Response) (*http.Response, error) {
	if resp == nil || resp.Body == nil {
		return resp, nil
	}
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if encoding != "gzip" {
		return resp, nil
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	resp.Body = decodedResponseBody{decoder: gz, body: resp.Body, reader: gz}
	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Content-Length")
	resp.ContentLength = -1
	return resp, nil
}

func preview(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 160 {
		return s[:160]
	}
	return s
}

func (c *Client) jsonHeaders(headers map[string]string) map[string]string {
	out := cloneStringMap(headers)
	out["Content-Type"] = "application/json"
	return out
}

func (c *Client) baseHeaders() map[string]string {
	out := cloneStringMap(dsprotocol.BaseHeaders)
	rangersID := ""
	if c != nil && c.Store != nil {
		rangersID = c.Store.DeepSeekRangersID()
	} else {
		rangersID = config.DeepSeekRangersIDFromEnv()
	}
	if strings.TrimSpace(rangersID) != "" {
		out["x-rangers-id"] = strings.TrimSpace(rangersID)
	}
	return out
}

func (c *Client) DeepSeekHeaders() map[string]string {
	return c.baseHeaders()
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
