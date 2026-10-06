package text

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	botTokenRE = regexp.MustCompile(`\b\d{6,}:[A-Za-z0-9_-]{20,}\b`)
	mcpTokenRE = regexp.MustCompile(`\bmcp_[A-Za-z0-9_-]+\b`)
)

// Truncate обрезает строку по рунам и помечает хвост.
func Truncate(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	note := "\n…[обрезано]"
	budget := max - utf8.RuneCountInString(note)
	if budget < 1 {
		budget = 1
	}
	runes := []rune(s)
	if budget > len(runes) {
		budget = len(runes)
	}
	return string(runes[:budget]) + note
}

// Split делит текст на части не длиннее limit рун, по возможности по переводам строк.
func Split(s string, limit int) []string {
	if limit < 1 {
		limit = 4096
	}
	if s == "" {
		return []string{""}
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return []string{s}
	}
	var parts []string
	for len(runes) > 0 {
		if len(runes) <= limit {
			parts = append(parts, string(runes))
			break
		}
		cut := limit
		window := runes[:cut]
		if i := lastNewline(window); i > limit/2 {
			cut = i + 1
		}
		parts = append(parts, string(runes[:cut]))
		runes = runes[cut:]
	}
	return parts
}

func lastNewline(rs []rune) int {
	for i := len(rs) - 1; i >= 0; i-- {
		if rs[i] == '\n' {
			return i
		}
	}
	return -1
}

// Redact скрывает mcp-токены и токены BotFather перед показом в чате.
func Redact(s string) string {
	s = botTokenRE.ReplaceAllString(s, "[токен скрыт]")
	s = mcpTokenRE.ReplaceAllString(s, "[mcp скрыт]")
	return s
}

// IsMCPToken сообщает, что сообщение целиком является токеном агента.
func IsMCPToken(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "mcp_") || len(s) < 8 {
		return false
	}
	return !strings.ContainsAny(s, " \t\r\n")
}
