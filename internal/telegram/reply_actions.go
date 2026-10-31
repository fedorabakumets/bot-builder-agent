// @fileoverview Открытые действия приветствия и раздела подключения.
package telegram

import (
	"bot-builder-agent/internal/telegram/router"
	"bot-builder-agent/internal/telegram/ui"
	"github.com/mymmrac/telego"
)

/** replyActions показывает основные действия сразу, остальные ответы сохраняют компактное меню. */
func replyActions(res router.Result) telego.ReplyMarkup {
	if res.ShowActions {
		return ui.ScreenActions(string(res.Next))
	}
	return screenMarkup(res.Next)
}
