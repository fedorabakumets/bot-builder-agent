// @fileoverview Проверка обращения по имени в группах и подписях к медиа.
package telegram

import (
	"github.com/mymmrac/telego"
	"testing"
)

// TestGroupAgentAddress проверяет границы обращения и сохранение фильтра обычных сообщений.
func TestGroupAgentAddress(t *testing.T) {
	for _, kind := range []string{telego.ChatTypeGroup, telego.ChatTypeSupergroup} {
		for _, caption := range []bool{false, true} {
			for _, tc := range []struct {
				text string
				want bool
			}{
				{"агент", true}, {" АГЕНТ, помоги", true}, {"Агент сделай бота", true}, {"агент: посмотри фото", true},
				{"агентство", false}, {"агенты", false}, {"агент123", false}, {"агент_бот", false},
				{"этот агент хорош", false}, {"/start@otherbot", false}, {"привет", false},
			} {
				msg := &telego.Message{Chat: telego.Chat{Type: kind}}
				if caption {
					msg.Caption = tc.text
				} else {
					msg.Text = tc.text
				}
				if got := directedAt(msg, telego.User{ID: 99, Username: "builder"}); got != tc.want {
					t.Errorf("%s: %q = %v", kind, tc.text, got)
				}
			}
		}
	}
}
