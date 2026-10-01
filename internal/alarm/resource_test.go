package alarm

import "testing"

func TestRequestConfigFromResponse(t *testing.T) {
	out := map[string]any{"data": map[string]any{"alarm": map[string]any{
		"id": "alarm-1", "metric_type": "api_errors", "threshold": float64(1),
		"enabled": true, "email_addresses": []any{"ops@example.com"},
		"created_at": "ignored",
	}}}
	cfg := requestConfigFromResponse(out)
	if cfg["metric_type"] != "api_errors" || cfg["threshold"] != float64(1) {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if _, ok := cfg["enabled"]; ok {
		t.Fatalf("response-only field leaked into config: %#v", cfg)
	}
	if _, ok := cfg["created_at"]; ok {
		t.Fatalf("server field leaked into config: %#v", cfg)
	}
}

func TestAlarmID(t *testing.T) {
	out := map[string]any{"data": map[string]any{"alarm": map[string]any{"id": "alarm-1"}}}
	if got := alarmID(out); got != "alarm-1" {
		t.Fatalf("got %q", got)
	}
}
