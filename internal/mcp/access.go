package mcp

import (
	"encoding/json"
	"strings"

	"bot-builder-agent/internal/text"
)

// ParseAccess разбирает токен, который выдаёт вкладка «Агент»:
// голый mcp_…, строку Bearer mcp_… или блок mcpServers с url и Authorization.
func ParseAccess(raw string) (token, endpoint string, ok bool) {
	raw = stripFence(strings.TrimSpace(raw))
	if raw == "" {
		return "", "", false
	}
	if text.IsMCPToken(raw) {
		return raw, "", true
	}
	if tok, ok := bearerValue(raw); ok {
		return tok, "", true
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return "", "", false
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw[start:end+1]), &doc); err != nil {
		return "", "", false
	}
	if serversRaw, exists := doc["mcpServers"]; exists {
		var servers map[string]json.RawMessage
		if err := json.Unmarshal(serversRaw, &servers); err != nil {
			return "", "", false
		}
		if rawServer, exists := servers["botcraft-builder"]; exists {
			if token, endpoint, ok = parseServer(rawServer); ok {
				return token, endpoint, true
			}
		}
		for _, rawServer := range servers {
			if token, endpoint, ok = parseServer(rawServer); ok {
				return token, endpoint, true
			}
		}
		return "", "", false
	}
	return parseServer(json.RawMessage(raw[start : end+1]))
}

func parseServer(raw json.RawMessage) (token, endpoint string, ok bool) {
	var srv struct {
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(raw, &srv); err != nil {
		return "", "", false
	}
	token = headerToken(srv.Headers)
	if token == "" {
		return "", "", false
	}
	if strings.TrimSpace(srv.URL) == "" {
		return token, "", true
	}
	normalized, err := NormalizeURL(srv.URL)
	if err != nil {
		return "", "", false
	}
	return token, normalized, true
}

func headerToken(headers map[string]string) string {
	for key, value := range headers {
		if strings.EqualFold(strings.TrimSpace(key), "authorization") {
			if token, ok := bearerValue(value); ok {
				return token
			}
		}
	}
	return ""
}

func bearerValue(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 7 && strings.EqualFold(raw[:7], "bearer ") {
		raw = strings.TrimSpace(raw[7:])
	}
	if text.IsMCPToken(raw) {
		return raw, true
	}
	return "", false
}

func stripFence(raw string) string {
	if !strings.HasPrefix(raw, "```") {
		return raw
	}
	raw = strings.TrimPrefix(raw, "```")
	if i := strings.IndexByte(raw, '\n'); i >= 0 {
		head := strings.TrimSpace(raw[:i])
		if head == "" || strings.EqualFold(head, "json") {
			raw = raw[i+1:]
		}
	}
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "```")
	return strings.TrimSpace(raw)
}
