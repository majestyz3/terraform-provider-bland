package alarms

import "testing"

func TestParseAlarms(t *testing.T) {
	out := map[string]any{"data": map[string]any{"alarms": []any{
		map[string]any{"id": "alarm-1", "name": "API errors", "metric_type": "api_errors"},
		map[string]any{"id": "alarm-2", "metric_type": "latency"},
	}}}
	items, names := parseAlarms(out)
	if len(items) != 2 || items[0].MetricType.ValueString() != "api_errors" {
		t.Fatalf("unexpected alarms: %#v", items)
	}
	if names["API errors"] != "alarm-1" {
		t.Fatalf("unexpected name map: %#v", names)
	}
}
