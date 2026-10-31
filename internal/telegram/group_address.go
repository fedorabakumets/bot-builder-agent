// @fileoverview Обращение «агент» в начале группового сообщения.
package telegram

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// agentAddress отличает обращение от составных слов, имён и обычного упоминания в середине фразы.
func agentAddress(text string) bool {
	text = strings.TrimSpace(text)
	const word = "агент"
	if len(text) < len(word) || !strings.EqualFold(text[:len(word)], word) {
		return false
	}
	rest := text[len(word):]
	if rest == "" {
		return true
	}
	next, _ := utf8.DecodeRuneInString(rest)
	return unicode.IsSpace(next) || strings.ContainsRune(",:;.!?—–-", next)
}
