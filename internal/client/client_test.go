package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoSendsAuthorizationAndDecodes(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("authorization"); got != "secret" {
			t.Fatalf("authorization=%q", got)
		}
		w.Header().Set("content-type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "abc"}, "errors": nil})
	}))
	defer s.Close()

	c := New(s.URL, "secret")
	out, status, err := c.Do(context.Background(), http.MethodGet, "/x", nil)
	if err != nil || status != 200 {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if out["data"] == nil {
		t.Fatalf("missing data")
	}
}

func TestDoTreatsApplicationErrorsAsFailure(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": nil,
			"errors": []any{map[string]any{"code": "BAD", "message": "nope"}},
		})
	}))
	defer s.Close()
	c := New(s.URL, "secret")
	if _, _, err := c.Do(context.Background(), http.MethodPost, "/x", map[string]any{}); err == nil {
		t.Fatal("expected application error")
	}
}
