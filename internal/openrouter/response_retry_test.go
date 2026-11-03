// @fileoverview Проверки пустых ответов, лимита токенов и сохранения контекста при повторе.
package openrouter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestEmptyResponseRetry сохраняет результат уже выполненного инструмента при повторе генерации.
func TestEmptyResponseRetry(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		messages := body["messages"].([]any)
		last := messages[len(messages)-1].(map[string]any)
		if last["role"] != "tool" || last["content"] != "saved revision 697" {
			t.Error("потерян результат инструмента")
		}
		w.Header().Set("Content-Type", "application/json")
		content := ""
		if calls == 2 {
			content = "Готово"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": content}}}})
	}))
	defer server.Close()
	client, _ := NewAt("test", server.URL, server.Client())
	out, err := client.Complete(context.Background(), Request{Model: "test", Messages: []Message{{Role: "tool", ToolCallID: "one", Content: "saved revision 697"}}})
	if err != nil || out.Content != "Готово" || calls != 2 {
		t.Fatalf("out=%v err=%v calls=%d", out, err, calls)
	}
}

// TestResponseClassification не повторяет вызов инструмента и различает остановку по лимиту.
func TestResponseClassification(t *testing.T) {
	cases := []struct {
		// name — название сценария.
		name string
		// body — ответ тестового провайдера.
		body string
		// code — ожидаемый код ошибки.
		code string
	}{
		{"tool", `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"x","type":"function","function":{"name":"db_add_node","arguments":"{}"}}]}}]}`, ""},
		{"length", `{"choices":[{"finish_reason":"length","message":{"role":"assistant","content":"часть ответа"}}]}`, "MODEL_TOKEN_LIMIT"},
		{"filter", `{"choices":[{"finish_reason":"content_filter","message":{"role":"assistant","content":""}}]}`, "MODEL_CONTENT_FILTER"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client, _ := NewAt("test", server.URL, server.Client())
			out, err := client.Complete(context.Background(), Request{Model: "test"})
			if calls != 1 {
				t.Fatalf("лишний повтор: %d", calls)
			}
			if tc.code == "" {
				if err != nil || len(out.ToolCalls) != 1 {
					t.Fatalf("out=%v err=%v", out, err)
				}
				return
			}
			var got *ResponseError
			if !errors.As(err, &got) || got.Code != tc.code {
				t.Fatal(err)
			}
		})
	}
}

// TestEmptyResponseExhaustion ограничивает число повторов даже при отсутствии choices.
func TestEmptyResponseExhaustion(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()
	client, _ := NewAt("test", server.URL, server.Client())
	_, err := client.Complete(context.Background(), Request{Model: "test"})
	var got *ResponseError
	if !errors.As(err, &got) || got.Code != "MODEL_EMPTY_RESPONSE" || got.Attempts != 3 || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
