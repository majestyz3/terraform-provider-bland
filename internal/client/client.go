package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func New(baseURL, apiKey string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.bland.ai"
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) Do(ctx context.Context, method, path string, body any) (map[string]any, int, error) {
	var lastStatus int
	for attempt := 0; attempt < 5; attempt++ {
		var reader io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				return nil, 0, err
			}
			reader = bytes.NewReader(b)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("authorization", c.APIKey)
		req.Header.Set("accept", "application/json")
		if body != nil {
			req.Header.Set("content-type", "application/json")
		}

		res, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, 0, err
		}
		lastStatus = res.StatusCode
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, lastStatus, readErr
		}

		out := map[string]any{}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &out); err != nil {
				if res.StatusCode >= 200 && res.StatusCode < 300 {
					return nil, res.StatusCode, fmt.Errorf("invalid JSON response: %w", err)
				}
				return nil, res.StatusCode, fmt.Errorf("Bland API %s: %s", res.Status, string(raw))
			}
		}

		if res.StatusCode >= 200 && res.StatusCode < 300 {
			if errorsVal, ok := out["errors"]; ok && errorsVal != nil {
				if arr, ok := errorsVal.([]any); ok && len(arr) > 0 {
					return out, res.StatusCode, fmt.Errorf("Bland API application errors: %s", compact(errorsVal))
				}
			}
			return out, res.StatusCode, nil
		}

		if retryable(res.StatusCode) && attempt < 4 {
			wait := time.Duration(attempt+1) * time.Second
			if v := res.Header.Get("Retry-After"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					wait = time.Duration(n) * time.Second
				}
			}
			select {
			case <-ctx.Done():
				return out, res.StatusCode, ctx.Err()
			case <-time.After(wait):
				continue
			}
		}
		return out, res.StatusCode, fmt.Errorf("Bland API returned %s: %s", res.Status, compact(out))
	}
	return nil, lastStatus, fmt.Errorf("Bland API request exhausted retries")
}

func retryable(code int) bool {
	return code == http.StatusRequestTimeout || code == http.StatusTooManyRequests || code >= 500
}

func compact(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
