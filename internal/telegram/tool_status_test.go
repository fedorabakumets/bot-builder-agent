/* @fileoverview Проверки сообщения Telegram с настоящим действием компактного MCP. */
package telegram

import (
	"context"
	"testing"

	"bot-builder-agent/internal/agent"
)

// TestActionStatusInTelegram проверяет реальный путь обновления статусного сообщения.
func TestActionStatusInTelegram(t *testing.T) {
	b, api, run := newTestBot(t)
	run.fn = func(_ context.Context, req agent.Request) (agent.Result, error) {
		req.OnTool("botcraft_builder__db_update_node")
		return agent.Result{Reply: "Готов ответ"}, nil
	}
	b.process(textUpdate(7, "Измени приветствие"))
	if !hasText(api.snapshot(), 7, "Изменяю ноду · botcraft_builder__db_update_node") {
		t.Fatalf("не показано настоящее действие: %+v", api.snapshot())
	}
}

// TestToolStatusFallback сохраняет имена discovery, полного профиля и сторонних инструментов.
func TestToolStatusFallback(t *testing.T) {
	for name, want := range map[string]string{
		"botcraft_builder__search_actions": "Ищу подходящее действие · botcraft_builder__search_actions",
		"db_update_node":                   "Изменяю ноду · db_update_node",
		"third_party__custom":              "Вызываю third_party__custom",
		"db_list_bots":                     "Вызываю db_list_bots",
	} {
		if got := toolStatus(name); got != want {
			t.Fatalf("%s: %q вместо %q", name, got, want)
		}
	}
}
