package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

// parseRPC разбирает одиночный JSON-RPC ответ или SSE-поток data:.
func parseRPC(contentType string, body []byte) (json.RawMessage, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, nil
	}
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") || looksLikeSSE(body) {
		payload, err := firstSSEData(body)
		if err != nil {
			return nil, err
		}
		body = payload
	}
	var resp rpcResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("битый JSON-RPC: %w", err)
	}
	if resp.Error != nil {
		msg := strings.TrimSpace(resp.Error.Message)
		if msg == "" {
			msg = "ошибка JSON-RPC"
		}
		return nil, fmt.Errorf("JSON-RPC: %s", msg)
	}
	if len(resp.Result) == 0 || string(resp.Result) == "null" {
		return nil, fmt.Errorf("битый JSON-RPC: нет result")
	}
	return resp.Result, nil
}

func looksLikeSSE(body []byte) bool {
	trim := bytes.TrimSpace(body)
	return bytes.HasPrefix(trim, []byte("data:")) || bytes.HasPrefix(trim, []byte("event:"))
}

func firstSSEData(body []byte) ([]byte, error) {
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var data strings.Builder
	flush := func() ([]byte, bool) {
		if data.Len() == 0 {
			return nil, false
		}
		return []byte(strings.TrimSpace(data.String())), true
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if payload, ok := flush(); ok {
				return payload, nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			chunk := strings.TrimPrefix(line, "data:")
			chunk = strings.TrimPrefix(chunk, " ")
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(chunk)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("SSE: %w", err)
	}
	if payload, ok := flush(); ok {
		return payload, nil
	}
	return nil, fmt.Errorf("битый JSON-RPC: пустой SSE")
}

type toolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

func joinContent(result json.RawMessage) (string, bool, error) {
	var parsed toolResult
	if err := json.Unmarshal(result, &parsed); err != nil {
		return "", false, fmt.Errorf("битый результат tools/call: %w", err)
	}
	var parts []string
	for _, block := range parsed.Content {
		if strings.TrimSpace(block.Text) == "" {
			continue
		}
		parts = append(parts, block.Text)
	}
	text := strings.Join(parts, "\n")
	if text == "" && parsed.IsError {
		text = "Инструмент вернул ошибку."
	}
	if text == "" {
		text = string(result)
	}
	return text, parsed.IsError, nil
}
