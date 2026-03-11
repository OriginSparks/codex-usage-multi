package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const endpointURL = "https://chatgpt.com/backend-api/wham/usage"

type Window struct {
	Label       string `json:"label"`
	UsedPercent int    `json:"usedPercent"`
	ResetAt     string `json:"resetAt,omitempty"`
}

type Snapshot struct {
	Profile        string   `json:"profile"`
	Plan           string   `json:"plan,omitempty"`
	CreditsBalance *float64 `json:"creditsBalance,omitempty"`
	Windows        []Window `json:"windows"`
	FetchedAt      string   `json:"fetchedAt"`
	AuthMode       string   `json:"authMode,omitempty"`
	Error          string   `json:"error,omitempty"`
}

func Fetch(profile, token string) (Snapshot, error) {
	req, err := http.NewRequest(http.MethodGet, endpointURL, nil)
	if err != nil {
		return Snapshot{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "codex-usage-multi")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Snapshot{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		if len(body) == 0 {
			return Snapshot{}, fmt.Errorf("usage request failed: HTTP %d", resp.StatusCode)
		}
		return Snapshot{}, fmt.Errorf("usage request failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Snapshot{}, err
	}

	snapshot := Snapshot{
		Profile:   profile,
		FetchedAt: NowUTC(),
		Windows:   []Window{},
	}
	if value, ok := raw["plan_type"].(string); ok {
		snapshot.Plan = value
	}
	if balance, ok := lookupFloat(raw, "credits", "balance"); ok {
		snapshot.CreditsBalance = &balance
	}
	if primary, ok := lookupMap(raw, "rate_limit", "primary_window"); ok {
		snapshot.Windows = append(snapshot.Windows, mapWindow(resolveLabel(primary, "primary"), primary))
	}
	if secondary, ok := lookupMap(raw, "rate_limit", "secondary_window"); ok {
		snapshot.Windows = append(snapshot.Windows, mapWindow(resolveLabel(secondary, "secondary"), secondary))
	}

	return snapshot, nil
}

func NowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func mapWindow(label string, raw map[string]any) Window {
	window := Window{
		Label:       label,
		UsedPercent: clampPercent(raw["used_percent"]),
	}
	if reset, ok := unixTimestamp(raw["reset_at"]); ok {
		window.ResetAt = time.Unix(reset, 0).UTC().Format(time.RFC3339)
	}
	return window
}

func clampPercent(input any) int {
	value, ok := floatValue(input)
	if !ok {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return int(value + 0.5)
}

func resolveLabel(raw map[string]any, fallback string) string {
	seconds, ok := lookupFloat(raw, "limit_window_seconds")
	if !ok || seconds <= 0 {
		return fallback
	}
	minutes := int(seconds / 60.0)
	switch minutes {
	case 300:
		return "5h"
	case 10080:
		return "1w"
	}
	hours := int(seconds/3600.0 + 0.5)
	if hours >= 24 && hours%24 == 0 {
		if hours == 24 {
			return "day"
		}
		return fmt.Sprintf("%dd", hours/24)
	}
	return fmt.Sprintf("%dh", hours)
}

func lookupMap(root map[string]any, keys ...string) (map[string]any, bool) {
	current := any(root)
	for _, key := range keys {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, ok := object[key]
		if !ok {
			return nil, false
		}
		current = value
	}
	object, ok := current.(map[string]any)
	return object, ok
}

func lookupFloat(root map[string]any, keys ...string) (float64, bool) {
	current := any(root)
	for i, key := range keys {
		object, ok := current.(map[string]any)
		if !ok {
			return 0, false
		}
		value, ok := object[key]
		if !ok {
			return 0, false
		}
		if i == len(keys)-1 {
			return floatValue(value)
		}
		current = value
	}
	return 0, false
}

func floatValue(input any) (float64, bool) {
	switch value := input.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		f, err := value.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(value, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func unixTimestamp(input any) (int64, bool) {
	switch value := input.(type) {
	case float64:
		return int64(value), true
	case int64:
		return value, true
	case int:
		return int64(value), true
	case json.Number:
		i, err := value.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}
