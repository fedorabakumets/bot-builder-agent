package text

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxTableColumns — предел Bot API: «Up to 20 columns in a table».
const maxTableColumns = 20

// sepCell — строка-разделитель GitHub-таблицы: ---, :---, ---:, :---:.
var sepCell = regexp.MustCompile(`^:?-{3,}:?$`)

// ToTelegramHTML переводит текст ответа в HTML.
// Сначала экранируются <, > и &, затем настоящие **жирный**, __жирный__,
// *курсив*, _курсив_, `код` и блоки ```. Идентификаторы вроде
// mcp_token_example и file_name.go не считаются курсивом.
// Markdown-таблицы с | становятся таблицей Rich HTML:
// <table bordered><tr><th>…</th></tr><tr><td>…</td></tr></table>.
// Этот фрагмент нельзя класть в sendMessage с parse_mode=HTML: обычный
// HTML-режим таблицу не умеет. Его отправляют методом sendRichMessage
// в поле rich_message.html (https://core.telegram.org/bots/api#rich-html-style).
// Текст ячеек экранируется, жирный, курсив и код внутри ячеек сохраняются.
// Таблица шире 20 колонок становится строками через « · », без пайпов.
// Если разметка получилась невалидной, возвращается экранированный
// текст без звёздочек и без пайп-таблицы.
func ToTelegramHTML(s string) string {
	var slots []string
	protected := protectCode(s, &slots)
	protected = protectTables(protected, &slots)
	out := emphasize(escapeTG(protected))
	out = restoreSlots(out, slots)
	if !balancedTelegramHTML(out) {
		return escapeTG(Plain(s))
	}
	return out
}

// Plain убирает markdown так, чтобы в чате не осталось ** и пайп-таблиц.
// Таблица становится строками «ячейка · ячейка». Жирный и курсив
// раскрываются в обычный текст, код теряет обратные кавычки.
func Plain(s string) string {
	var slots []string
	protected := protectCodeAs(s, &slots, false)
	protected = plainTables(protected)
	protected = unwrapEmphasis(protected)
	return restoreSlots(protected, slots)
}

// WrapRichHTML собирает rich_message.html: абзацы в <p>, переносы внутри
// абзаца в <br>, таблица и <pre> остаются отдельными блоками.
func WrapRichHTML(s string) string {
	var b strings.Builder
	for len(s) > 0 {
		rel, kind := nextRichAtom(s)
		if rel < 0 {
			b.WriteString(wrapParagraphs(s))
			break
		}
		if rel > 0 {
			if w := wrapParagraphs(s[:rel]); w != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(w)
			}
		}
		endTag := "</pre>"
		if kind == "table" {
			endTag = "</table>"
		}
		relEnd := strings.Index(s[rel:], endTag)
		if relEnd < 0 {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(s[rel:])
			break
		}
		end := rel + relEnd + len(endTag)
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(s[rel:end])
		s = s[end:]
	}
	return strings.Trim(b.String(), "\n")
}

func nextRichAtom(s string) (int, string) {
	table := strings.Index(s, "<table")
	pre := strings.Index(s, "<pre>")
	switch {
	case table < 0 && pre < 0:
		return -1, ""
	case table < 0:
		return pre, "pre"
	case pre < 0:
		return table, "table"
	case table <= pre:
		return table, "table"
	default:
		return pre, "pre"
	}
}

func wrapParagraphs(s string) string {
	s = strings.Trim(s, "\n")
	if strings.TrimSpace(s) == "" {
		return ""
	}
	var b strings.Builder
	for _, para := range strings.Split(s, "\n\n") {
		para = strings.Trim(para, "\n")
		if strings.TrimSpace(para) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("<p>")
		b.WriteString(strings.ReplaceAll(para, "\n", "<br>"))
		b.WriteString("</p>")
	}
	return b.String()
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
	return protectCodeAs(s, slots, true)
}

func protectCodeAs(s string, slots *[]string, asHTML bool) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if code, next, ok := readFence(s, i); ok {
			body := code
			if asHTML {
				body = "<pre>" + escapeTG(code) + "</pre>"
			}
			b.WriteString(putSlot(slots, body))
			i = next
			continue
		}
		if s[i] == '`' {
			if code, next, ok := readInline(s, i); ok {
				body := code
				if asHTML {
					body = "<code>" + escapeTG(code) + "</code>"
				}
				b.WriteString(putSlot(slots, body))
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

func unwrapEmphasis(s string) string {
	s = replaceMarked(s, "**", "", "", false)
	s = replaceMarked(s, "__", "", "", true)
	s = replaceMarked(s, "*", "", "", false)
	s = replaceMarked(s, "_", "", "", true)
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

// protectTables заменяет markdown-таблицы на слот с HTML <table>.
// Ячейки уже экранированы и прогнаны через жирный/курсив/код.
func protectTables(s string, slots *[]string) string {
	if !strings.Contains(s, "|") {
		return s
	}
	lines := strings.Split(s, "\n")
	var b strings.Builder
	changed := false
	for i := 0; i < len(lines); {
		rows, aligns, n, ok := parsePipeTable(lines, i)
		if ok {
			var html string
			if len(rows[0]) > maxTableColumns {
				html = renderPlainRows(rows, *slots, true)
			} else {
				html = renderTable(rows, aligns, *slots)
			}
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(putSlot(slots, html))
			i += n
			changed = true
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(lines[i])
		i++
	}
	if !changed {
		return s
	}
	return b.String()
}

func parsePipeTable(lines []string, start int) (rows [][]string, aligns []string, n int, ok bool) {
	if start+1 >= len(lines) {
		return nil, nil, 0, false
	}
	header, ok := pipeCells(lines[start])
	if !ok || len(header) == 0 || isSeparator(header) {
		return nil, nil, 0, false
	}
	sep, ok := pipeCells(lines[start+1])
	if !ok || len(sep) != len(header) || !isSeparator(sep) {
		return nil, nil, 0, false
	}
	aligns = make([]string, len(sep))
	for i, cell := range sep {
		aligns[i] = cellAlign(cell)
	}
	rows = [][]string{header}
	n = 2
	for start+n < len(lines) {
		cells, ok := pipeCells(lines[start+n])
		if !ok || len(cells) != len(header) || isSeparator(cells) {
			break
		}
		rows = append(rows, cells)
		n++
	}
	return rows, aligns, n, true
}

func cellAlign(sep string) string {
	left := strings.HasPrefix(sep, ":")
	right := strings.HasSuffix(sep, ":")
	switch {
	case left && right:
		return "center"
	case right:
		return "right"
	case left:
		return "left"
	default:
		return ""
	}
}

func pipeCells(line string) ([]string, bool) {
	trim := strings.TrimSpace(line)
	if !strings.Contains(trim, "|") {
		return nil, false
	}
	trim = strings.ReplaceAll(trim, `\|`, "\x01")
	if strings.HasPrefix(trim, "|") {
		trim = strings.TrimPrefix(trim, "|")
	}
	if strings.HasSuffix(trim, "|") {
		trim = strings.TrimSuffix(trim, "|")
	}
	parts := strings.Split(trim, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.ReplaceAll(strings.TrimSpace(p), "\x01", "|")
	}
	return cells, true
}

func isSeparator(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		if !sepCell.MatchString(c) {
			return false
		}
	}
	return true
}

func renderTable(rows [][]string, aligns []string, slots []string) string {
	var b strings.Builder
	b.WriteString("<table bordered>")
	for r, row := range rows {
		tag := "td"
		if r == 0 {
			tag = "th"
		}
		b.WriteString("<tr>")
		for c, cell := range row {
			body := restoreSlots(emphasize(escapeTG(cell)), slots)
			b.WriteByte('<')
			b.WriteString(tag)
			if c < len(aligns) && aligns[c] != "" {
				b.WriteString(` align="`)
				b.WriteString(aligns[c])
				b.WriteByte('"')
			}
			b.WriteByte('>')
			b.WriteString(body)
			b.WriteString("</")
			b.WriteString(tag)
			b.WriteByte('>')
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</table>")
	return b.String()
}

func plainTables(s string) string {
	if !strings.Contains(s, "|") {
		return s
	}
	lines := strings.Split(s, "\n")
	var b strings.Builder
	changed := false
	for i := 0; i < len(lines); {
		rows, _, n, ok := parsePipeTable(lines, i)
		if ok {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(renderPlainRows(rows, nil, false))
			i += n
			changed = true
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(lines[i])
		i++
	}
	if !changed {
		return s
	}
	return b.String()
}

func renderPlainRows(rows [][]string, slots []string, asHTML bool) string {
	var b strings.Builder
	for r, row := range rows {
		if r > 0 {
			b.WriteByte('\n')
		}
		for c, cell := range row {
			if c > 0 {
				b.WriteString(" · ")
			}
			if asHTML {
				b.WriteString(restoreSlots(emphasize(escapeTG(cell)), slots))
				continue
			}
			b.WriteString(unwrapEmphasis(cell))
		}
	}
	return b.String()
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
		if sp := strings.IndexAny(tag, " \t/"); sp >= 0 {
			tag = tag[:sp]
		}
		switch tag {
		case "b", "i", "code", "pre", "table", "tr", "th", "td":
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
