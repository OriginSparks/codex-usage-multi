package usage

import "testing"

func TestResolveLabelMapsOfficialWindows(t *testing.T) {
	if got := resolveLabel(map[string]any{"limit_window_seconds": float64(300 * 60)}, "primary"); got != "5h" {
		t.Fatalf("expected 5h, got %q", got)
	}
	if got := resolveLabel(map[string]any{"limit_window_seconds": float64(10080 * 60)}, "secondary"); got != "1w" {
		t.Fatalf("expected 1w, got %q", got)
	}
}
