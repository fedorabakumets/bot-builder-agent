package telegram

import (
	"strings"
	"unicode/utf16"

	"github.com/mymmrac/telego"
)

// directedAt сообщает, нужно ли отвечать на сообщение.
// В личке — да. В группе и супергруппе — только команда этому боту,
// упоминание или ответ на его сообщение. Остальные реплики пропускаются.
func directedAt(msg *telego.Message, me telego.User) bool {
	if msg == nil {
		return true
	}
	switch msg.Chat.Type {
	case telego.ChatTypeGroup, telego.ChatTypeSupergroup:
	default:
		return true
	}
	if me.ID != 0 && msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.ID == me.ID {
		return true
	}
	if commandTargets(msg.Text, me.Username) || commandTargets(msg.Caption, me.Username) {
		return true
	}
	if hasMention(msg.Text, msg.Entities, me) || hasMention(msg.Caption, msg.CaptionEntities, me) {
		return true
	}
	return false
}

// commandTargets принимает /cmd и /cmd@этотбот. /cmd@другойбот — нет.
func commandTargets(text, username string) bool {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return false
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}
	head := fields[0]
	at := strings.IndexByte(head, '@')
	if at < 0 {
		return len(strings.TrimPrefix(head, "/")) > 0
	}
	target := head[at+1:]
	return username != "" && strings.EqualFold(target, username) && len(head[1:at]) > 0
}

func hasMention(text string, entities []telego.MessageEntity, me telego.User) bool {
	want := "@" + me.Username
	for _, entity := range entities {
		switch entity.Type {
		case telego.EntityTypeTextMention:
			if entity.User != nil && me.ID != 0 && entity.User.ID == me.ID {
				return true
			}
		case telego.EntityTypeMention:
			got := entityText(text, entity.Offset, entity.Length)
			if me.Username != "" && strings.EqualFold(got, want) {
				return true
			}
		}
	}
	return false
}

// entityText вырезает кусок по смещению Telegram: UTF-16, не байты и не руны.
func entityText(text string, offset, length int) string {
	if offset < 0 || length <= 0 {
		return ""
	}
	units := utf16.Encode([]rune(text))
	end := offset + length
	if end > len(units) {
		return ""
	}
	return string(utf16.Decode(units[offset:end]))
}
