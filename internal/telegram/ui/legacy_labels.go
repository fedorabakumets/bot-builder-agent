// @fileoverview Совместимость с кнопками ранее отправленных меню.
package ui

/** NormalizeMenuText сохраняет действия старых reply-клавиатур после обновления подписей. */
func NormalizeMenuText(value string) string {
	switch value {
	case "💬 Задача":
		return BtnTask
	case "⚙️ Аккаунт":
		return BtnAccount
	case "🔑 MCP-токен":
		return BtnToken
	case "🧹 Сбросить диалог":
		return BtnReset
	case "🚪 Выйти":
		return BtnLogout
	default:
		return value
	}
}
