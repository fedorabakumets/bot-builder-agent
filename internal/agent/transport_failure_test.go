// @fileoverview Проверка остановки цикла модели после неоднозначного сбоя изменения.
package agent

import (
	"bot-builder-agent/internal/openrouter"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// failedMutation имитирует обрыв после отправки изменения серверу.
type failedMutation struct{ calls int }

// Catalog возвращает минимальный набор для проверки.
func (f *failedMutation) Catalog(context.Context, int64) (RemoteSet, error) { return RemoteSet{}, nil }

// Call фиксирует отправку и возвращает транспортную ошибку.
func (f *failedMutation) Call(context.Context, int64, string, json.RawMessage) (string, bool, error) {
	f.calls++
	return "", false, errors.New("connection reset")
}

// TestTransportFailureStopsLoop не возвращает ошибку модели как повод повторить изменение.
func TestTransportFailureStopsLoop(t *testing.T) {
	remote := &failedMutation{}
	runner := Runner{Remotes: remote}
	calls := []openrouter.ToolCall{{ID: "call", Name: "builder__write", Arguments: "{}"}}
	out, err := runner.execTools(context.Background(), Request{UserID: 42}, calls, 1000, map[string]Remote{"builder__write": {Builder: true, Original: "write"}})
	if err == nil || out != nil || remote.calls != 1 {
		t.Fatalf("ошибка %v, ответ %+v, вызовов %d", err, out, remote.calls)
	}
}
