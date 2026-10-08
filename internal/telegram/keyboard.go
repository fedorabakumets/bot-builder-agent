package telegram

import "github.com/mymmrac/telego"

// replyHideText — текст служебного сообщения, которым снимается reply-клавиатура.
// Сообщение сразу удаляется: у sendMessage одно поле reply_markup, а кнопки
// уже висят на основном тексте.
const replyHideText = "·"

// dropReplyKeyboard один раз на чат прячет залипшую reply-клавиатуру.
// Повторно не шлёт, чтобы в чате не мелькала точка на каждом ответе.
func (b *Bot) dropReplyKeyboard(chatID int64) {
	if !b.claimKeyboardHide(chatID) {
		return
	}
	msg, err := b.api.SendMessage(b.baseCtx(), &telego.SendMessageParams{
		ChatID:              telego.ChatID{ID: chatID},
		Text:                replyHideText,
		DisableNotification: true,
		ReplyMarkup:         &telego.ReplyKeyboardRemove{RemoveKeyboard: true},
	})
	if err != nil || msg == nil || msg.MessageID == 0 {
		b.releaseKeyboardHide(chatID)
		if err != nil {
			b.log.Warn("chat=%d: не удалось снять клавиатуру: %v", chatID, err)
		}
		return
	}
	if err := b.api.DeleteMessage(b.baseCtx(), &telego.DeleteMessageParams{
		ChatID:    telego.ChatID{ID: chatID},
		MessageID: msg.MessageID,
	}); err != nil {
		b.log.Warn("chat=%d: служебное сообщение осталось: %v", chatID, err)
	}
}

func (b *Bot) claimKeyboardHide(chatID int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.keyboardHidden == nil {
		b.keyboardHidden = map[int64]struct{}{}
	}
	if _, ok := b.keyboardHidden[chatID]; ok {
		return false
	}
	b.keyboardHidden[chatID] = struct{}{}
	return true
}

func (b *Bot) releaseKeyboardHide(chatID int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.keyboardHidden, chatID)
}
