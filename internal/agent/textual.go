package agent

import (
	"strings"
	"unicode"
)

// DetectTextualCall ловит вызов, который модель написала текстом: name({...}).
// known проверяет, что имя есть в списке инструментов. Имена проверяются от длинных к коротким.
func DetectTextualCall(content string, names []string) (name, args string, ok bool) {
	c := strings.TrimSpace(content)
	if c == "" || len(names) == 0 {
		return "", "", false
	}
	ordered := append([]string(nil), names...)
	sortByLenDesc(ordered)
	lower := strings.ToLower(c)
	for _, n := range ordered {
		ln := strings.ToLower(n)
		if !strings.HasPrefix(lower, ln) {
			continue
		}
		rest := strings.TrimLeftFunc(c[len(n):], unicode.IsSpace)
		if rest == "" {
			continue
		}
		switch rest[0] {
		case '(', '{', ':', '[':
		default:
			continue
		}
		jsonStart := strings.Index(rest, "{")
		if jsonStart < 0 {
			continue
		}
		obj, end, found := extractObject(rest[jsonStart:])
		if !found {
			continue
		}
		tail := strings.TrimSpace(rest[jsonStart+end:])
		tail = strings.Trim(tail, " )]")
		if tail != "" {
			continue
		}
		return n, obj, true
	}
	return "", "", false
}

// LooksLikeRawToolCall говорит, что текст похож на сырой вызов, а не на ответ.
func LooksLikeRawToolCall(content string, names []string) bool {
	_, _, ok := DetectTextualCall(content, names)
	return ok
}

func sortByLenDesc(names []string) {
	for i := 1; i < len(names); i++ {
		j := i
		for j > 0 && len(names[j]) > len(names[j-1]) {
			names[j], names[j-1] = names[j-1], names[j]
			j--
		}
	}
}

func extractObject(s string) (string, int, bool) {
	if s == "" || s[0] != '{' {
		return "", 0, false
	}
	depth := 0
	inStr := false
	esc := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if ch == '\\' {
				esc = true
				continue
			}
			if ch == '"' {
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[:i+1], i + 1, true
			}
		}
	}
	return "", 0, false
}
