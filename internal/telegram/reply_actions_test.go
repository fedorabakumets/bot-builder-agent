// @fileoverview Проверка доступности основных действий и совместимости старых меню.
package telegram

import (
	"bot-builder-agent/internal/telegram/router"
	"bot-builder-agent/internal/telegram/ui"
	"testing"
)

/** TestWelcomeActions проверяет, что приветствие сразу предлагает рабочие кнопки. */
func TestWelcomeActions(t *testing.T) {
	for _, command := range []string{"/start", "/help"} {
		result := router.Handle(router.Input{Text: command, Allowed: true})
		keyboard := ui.InlineForGroup(replyActions(result))
		if keyboard == nil || len(keyboard.InlineKeyboard) != 3 {
			t.Fatalf("%s: основные действия скрыты", command)
		}
		for _, row := range keyboard.InlineKeyboard {
			for _, button := range row {
				action := router.Handle(router.Input{Callback: button.CallbackData, Allowed: true, HasToken: true})
				if action.Kind == router.KindAskAgent {
					t.Fatalf("кнопка %s стала задачей модели", button.Text)
				}
			}
		}
	}
}

/** TestLegacyMenuActions не позволяет старым кнопкам превратиться в запросы модели. */
func TestLegacyMenuActions(t *testing.T) {
	for _, label := range []string{"💬 Задача", "⚙️ Аккаунт", "🔑 MCP-токен", "🧹 Сбросить диалог", "🚪 Выйти"} {
		result := router.Handle(router.Input{Text: label, Allowed: true, HasToken: true})
		if result.Kind == router.KindAskAgent {
			t.Fatalf("старая кнопка %s стала задачей модели", label)
		}
	}
}
