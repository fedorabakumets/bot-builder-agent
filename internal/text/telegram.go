package text

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ToTelegramHTML переводит текст ответа в HTML для parse_mode=HTML.
// Сначала экранируются <, > и &, затем настоящие **жирный**, __жирный__,
// *курсив*, _курсив_, `код` и блоки ```. Идентификаторы вроде
// mcp_token_example и file_name.go не считаются курсивом.
// Если разметка получилась невалидной, возвращается полностью экранированный
// текст без тегов — его всё ещё можно отправить как HTML.
func ToTelegramHTML(s string) string {
	var slots []string
	protected := protectCode(s, &slots)
	out := emphasize(escapeTG(protected))
	out = restoreSlots(out, slots)
	if !balancedTelegramHTML(out) {
		return escapeTG(s)
	}
	return out
}

func escapeTG(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func protectCode(s string, slots *[]string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if code, next, ok := readFence(s, i); ok {
			b.WriteString(putSlot(slots, "<pre>"+escapeTG(code)+"</pre>"))
			i = next
			continue
		}
		if s[i] == '`' {
			if code, next, ok := readInline(s, i); ok {
				b.WriteString(putSlot(slots, "<code>"+escapeTG(code)+"</code>"))
				i = next
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func putSlot(slots *[]string, html string) string {
	id := len(*slots)
	*slots = append(*slots, html)
	return "\x00TG" + itoa(id) + "\x00"
}

func restoreSlots(s string, slots []string) string {
	for i, html := range slots {
		s = strings.ReplaceAll(s, "\x00TG"+itoa(i)+"\x00", html)
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func readFence(s string, i int) (code string, next int, ok bool) {
	if !strings.HasPrefix(s[i:], "```") {
		return "", 0, false
	}
	if i > 0 && s[i-1] != '\n' {
		return "", 0, false
	}
	lineEnd := strings.IndexByte(s[i:], '\n')
	if lineEnd < 0 {
		return "", 0, false
	}
	lang := strings.TrimSpace(strings.TrimPrefix(s[i:i+lineEnd], "```"))
	if lang != "" && !fenceLang(lang) {
		return "", 0, false
	}
	contentStart := i + lineEnd + 1
	rel := findCloseFence(s[contentStart:])
	if rel < 0 {
		return "", 0, false
	}
	code = strings.TrimSuffix(s[contentStart:contentStart+rel], "\n")
	code = strings.TrimSuffix(code, "\r")
	next = contentStart + rel + 3
	if nl := strings.IndexByte(s[next:], '\n'); nl >= 0 && strings.TrimSpace(s[next:next+nl]) == "" {
		next += nl
	} else if strings.TrimSpace(s[next:]) == "" {
		next = len(s)
	}
	return code, next, true
}

func fenceLang(s string) bool {
	if s == "" || len(s) > 32 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '_' || r == '+' || r == '.' || r == '#' || r == '-':
		default:
			return false
		}
	}
	return true
}

func findCloseFence(s string) int {
	i := 0
	for i <= len(s) {
		if (i == 0 || (i > 0 && s[i-1] == '\n')) && strings.HasPrefix(s[i:], "```") {
			rest := s[i+3:]
			lineEnd := strings.IndexByte(rest, '\n')
			line := rest
			if lineEnd >= 0 {
				line = rest[:lineEnd]
			}
			if strings.TrimSpace(line) == "" {
				return i
			}
		}
		if i >= len(s) {
			break
		}
		nl := strings.IndexByte(s[i:], '\n')
		if nl < 0 {
			break
		}
		i += nl + 1
	}
	return -1
}

func readInline(s string, i int) (string, int, bool) {
	if i >= len(s) || s[i] != '`' {
		return "", 0, false
	}
	rel := strings.IndexByte(s[i+1:], '`')
	if rel <= 0 {
		return "", 0, false
	}
	content := s[i+1 : i+1+rel]
	if strings.ContainsAny(content, "\n\r") {
		return "", 0, false
	}
	return content, i + 1 + rel + 1, true
}

func emphasize(s string) string {
	s = replaceMarked(s, "**", "<b>", "</b>", false)
	s = replaceMarked(s, "__", "<b>", "</b>", true)
	s = replaceMarked(s, "*", "<i>", "</i>", false)
	s = replaceMarked(s, "_", "<i>", "</i>", true)
	return s
}

func replaceMarked(s, mark, openTag, closeTag string, underscore bool) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if !strings.HasPrefix(s[i:], mark) {
			b.WriteByte(s[i])
			i++
			continue
		}
		if len(mark) == 1 && strings.HasPrefix(s[i:], mark+mark) {
			b.WriteByte(s[i])
			i++
			continue
		}
		if !delimEdge(s, i, true, underscore) {
			b.WriteByte(s[i])
			i++
			continue
		}
		rest := s[i+len(mark):]
		closeRel := strings.Index(rest, mark)
		if closeRel < 0 {
			b.WriteByte(s[i])
			i++
			continue
		}
		inner := rest[:closeRel]
		end := i + len(mark) + closeRel + len(mark)
		if !validInner(inner) || !delimEdge(s, end, false, underscore) {
			b.WriteByte(s[i])
			i++
			continue
		}
		b.WriteString(openTag)
		b.WriteString(inner)
		b.WriteString(closeTag)
		i = end
	}
	return b.String()
}

func validInner(s string) bool {
	if s == "" || strings.ContainsAny(s, "\r\n<>") {
		return false
	}
	first, _ := utf8.DecodeRuneInString(s)
	last, _ := utf8.DecodeLastRuneInString(s)
	return !unicode.IsSpace(first) && !unicode.IsSpace(last)
}

func delimEdge(s string, index int, before, underscore bool) bool {
	var r rune
	var size int
	if before {
		if index == 0 {
			return true
		}
		r, _ = utf8.DecodeLastRuneInString(s[:index])
	} else {
		if index >= len(s) {
			return true
		}
		r, size = utf8.DecodeRuneInString(s[index:])
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return false
	}
	if underscore {
		if r == '_' || r == '/' || r == '\\' || (before && r == '.') {
			return false
		}
		if !before && r == '.' && index+size < len(s) {
			n, _ := utf8.DecodeRuneInString(s[index+size:])
			if unicode.IsLetter(n) || unicode.IsDigit(n) {
				return false
			}
		}
		return true
	}
	return r != '*'
}

func balancedTelegramHTML(s string) bool {
	var stack []string
	for i := 0; i < len(s); {
		if s[i] != '<' {
			i++
			continue
		}
		j := strings.IndexByte(s[i:], '>')
		if j < 0 {
			return false
		}
		tag := s[i+1 : i+j]
		i += j + 1
		closing := strings.HasPrefix(tag, "/")
		if closing {
			tag = strings.TrimPrefix(tag, "/")
		}
		switch tag {
		case "b", "i", "code", "pre":
		default:
			return false
		}
		if !closing {
			stack = append(stack, tag)
			continue
		}
		if len(stack) == 0 || stack[len(stack)-1] != tag {
			return false
		}
		stack = stack[:len(stack)-1]
	}
	return len(stack) == 0
}
