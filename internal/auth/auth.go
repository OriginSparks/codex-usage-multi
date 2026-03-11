package auth

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
)

var skipKeyPattern = regexp.MustCompile(`(?i)(refresh|secret|api[_-]?key)`)

func ExtractBearerTokenFromFile(path string) (token, mode string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}

	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", "", err
	}

	root, ok := decoded.(map[string]any)
	if !ok {
		return "", "", errors.New("auth.json does not contain a JSON object")
	}

	if value, ok := root["auth_mode"].(string); ok {
		mode = strings.TrimSpace(value)
	}
	if value, ok := root["OPENAI_API_KEY"].(string); ok && strings.TrimSpace(value) != "" {
		return "", mode, errors.New("this profile appears to be API-key based. The usage endpoint needs a ChatGPT/Codex bearer token, not OPENAI_API_KEY")
	}

	if token, ok := searchToken(root["tokens"]); ok {
		return token, mode, nil
	}
	if token, ok := searchToken(root); ok {
		return token, mode, nil
	}

	return "", mode, errors.New("could not find a usable bearer token in auth.json. The auth format may have changed; inspect the file manually and update the adapter")
}

func searchToken(input any) (string, bool) {
	switch value := input.(type) {
	case []any:
		for _, item := range value {
			if token, ok := searchToken(item); ok {
				return token, true
			}
		}
	case map[string]any:
		preferred := []string{
			"access_token",
			"accessToken",
			"token",
			"id_token",
			"idToken",
			"chatgpt_access_token",
		}
		for _, key := range preferred {
			if token := maybeToken(value[key]); token != "" {
				return token, true
			}
		}
		for key, child := range value {
			if skipKeyPattern.MatchString(key) {
				continue
			}
			if token, ok := searchToken(child); ok {
				return token, true
			}
		}
	}
	return "", false
}

func maybeToken(input any) string {
	text, ok := input.(string)
	if !ok {
		return ""
	}
	text = strings.TrimSpace(text)
	if strings.HasPrefix(strings.ToLower(text), "bearer ") {
		text = strings.TrimSpace(text[7:])
	}
	if len(text) < 20 || strings.ContainsRune(text, ' ') {
		return ""
	}
	return text
}
