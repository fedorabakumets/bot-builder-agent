/* @fileoverview Проверки статуса реального действия и неизменности исполняемого вызова MCP. */
package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"bot-builder-agent/internal/openrouter"
)

// progressRemote сохраняет фактический вызов без обращения к сети.
type progressRemote struct {
	// name — исполняемое имя опубликованного инструмента.
	name string
	// args — исходные аргументы, не предназначенные для статуса.
	args json.RawMessage
}

// Catalog реализует интерфейс каталога, не используемый при прямом прогоне выполнения.
func (r *progressRemote) Catalog(context.Context, int64) (RemoteSet, error) { return RemoteSet{}, nil }

// Call записывает единственный вызов транспорта.
func (r *progressRemote) Call(_ context.Context, _ int64, name string, args json.RawMessage) (string, bool, error) {
	r.name, r.args = name, args
	return `{"ok":true}`, false, nil
}

// TestProgressKeepsExecution проверяет, что развёрнуто только отображение, а не вызов транспорта.
func TestProgressKeepsExecution(t *testing.T) {
	remote := &progressRemote{}
	runner := Runner{Remotes: remote}
	name := "botcraft_builder__call_action"
	args := `{"action_id":"db_update_node","args":{"project_id":7,"private_value":"do-not-show"}}`
	var displayed string
	outs, err := runner.execTools(context.Background(), Request{OnTool: func(name string) { displayed = name }},
		[]openrouter.ToolCall{{ID: "one", Name: name, Arguments: args}}, 1000,
		map[string]Remote{name: {Original: "call_action", Builder: true}})
	if err != nil || len(outs) != 1 {
		t.Fatalf("выполнение: outs=%v err=%v", outs, err)
	}
	if displayed != "botcraft_builder__db_update_node" || strings.Contains(displayed, "do-not-show") {
		t.Fatalf("статус: %q", displayed)
	}
	if remote.name != name || string(remote.args) != args || outs[0].name != name {
		t.Fatalf("изменён реальный вызов: %s %s", remote.name, remote.args)
	}
}

// TestProgressDoesNotTrustOtherServers исключает подмену статуса чужим или некорректным action_id.
func TestProgressDoesNotTrustOtherServers(t *testing.T) {
	name := "other__call_action"
	args := `{"action_id":"db_delete_project","args":{"secret":"do-not-show"}}`
	if got := toolProgressName(name, args, map[string]Remote{name: {Original: "call_action"}}); got != name {
		t.Fatal(got)
	}
	for _, raw := range []string{`not-json`, `{"action_id":7}`, `{"action_id":"db_update_node\nSECRET"}`, `{"action_id":"<b>fake</b>"}`, `{"action_id":"mcp_private_token"}`} {
		if got := toolProgressName("call_action", raw, nil); got != "call_action" {
			t.Fatalf("небезопасное имя %q", got)
		}
	}
	if got := toolProgressName("db_list_projects", `{}`, nil); got != "db_list_projects" {
		t.Fatal(got)
	}
}
