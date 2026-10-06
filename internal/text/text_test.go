package text

import (
	"strings"
	"testing"
)

func TestTruncateAndSplit(t *testing.T) {
	got := Truncate("абвгд", 4)
	if got == "абвгд" || len([]rune(got)) > 4+len([]rune("\n…[обрезано]")) {
		t.Fatal(got)
	}
	if !strings.Contains(got, "обрезано") {
		t.Fatal(got)
	}
	parts := Split("one\ntwo\nthree", 8)
	if len(parts) < 2 {
		t.Fatalf("%q", parts)
	}
	for _, p := range parts {
		if len([]rune(p)) > 8 {
			t.Fatalf("длинный кусок %q", p)
		}
	}
	long := make([]rune, 5000)
	for i := range long {
		long[i] = 'я'
	}
	chunks := Split(string(long), 4096)
	if len(chunks) != 2 {
		t.Fatalf("chunks %d", len(chunks))
	}
}

func TestToTelegramHTML(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{name: "bold stars", in: "**Что было:**", want: "<b>Что было:</b>"},
		{name: "bold under", in: "__жирный__", want: "<b>жирный</b>"},
		{name: "italic star", in: "это *важно*", want: "это <i>важно</i>"},
		{name: "italic under", in: "это _важно_", want: "это <i>важно</i>"},
		{name: "inline code", in: "см. `file_name.go`", want: "см. <code>file_name.go</code>"},
		{name: "plain", in: "Обычный ответ без разметки.", want: "Обычный ответ без разметки."},
		{name: "snake and file", in: "mcp_token_example и file_name.go", want: "mcp_token_example и file_name.go"},
		{name: "dunder file", in: "модуль __init__.py", want: "модуль __init__.py"},
		{name: "lone star", in: "2 * 3 = 6", want: "2 * 3 = 6"},
		{name: "escape", in: "a < b & c", want: "a &lt; b &amp; c"},
		{name: "code protects stars", in: "пример `**не жирный**`", want: "пример <code>**не жирный**</code>"},
		{name: "raw html escaped", in: "смотри <b>нет</b> и **да**", want: "смотри &lt;b&gt;нет&lt;/b&gt; и <b>да</b>"},
		{name: "several", in: "**a** and __b__ and *c* and _d_", want: "<b>a</b> and <b>b</b> and <i>c</i> and <i>d</i>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ToTelegramHTML(tc.in)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestToTelegramHTMLModelAnswer(t *testing.T) {
	in := strings.Join([]string{
		"**Что было:**",
		"* бот показывал **звёзды** в чате",
		"* файл __init__.py, mcp_token_example и file_name.go",
		"",
		"**Что сделано:**",
		"Обернул *важное* и оставил `snake_case`.",
		"",
		"```go",
		`fmt.Println("a < b")`,
		"```",
	}, "\n")
	want := strings.Join([]string{
		"<b>Что было:</b>",
		"* бот показывал <b>звёзды</b> в чате",
		"* файл __init__.py, mcp_token_example и file_name.go",
		"",
		"<b>Что сделано:</b>",
		"Обернул <i>важное</i> и оставил <code>snake_case</code>.",
		"",
		`<pre>fmt.Println("a &lt; b")</pre>`,
	}, "\n")
	got := ToTelegramHTML(in)
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "**") {
		t.Fatalf("raw bold markers left: %s", got)
	}
}

func TestToTelegramHTMLFence(t *testing.T) {
	in := "до\n```\n**не жирный** и a < b\n```\nпосле"
	got := ToTelegramHTML(in)
	want := "до\n<pre>**не жирный** и a &lt; b</pre>\nпосле"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestToTelegramHTMLTorLinkTable(t *testing.T) {
	in := strings.Join([]string{
		"Все 4 бота проекта TorLink уже работают",
		"",
		"| Бот | Username | Аптайм | Всего | За 24ч | Новые сегодня |",
		"|-----|----------|--------|-------|--------|---------------|",
		"| TorLink | @TorLink_brobot | 22ч 37м | 586 | 46 | 44 |",
	}, "\n")
	want := strings.Join([]string{
		"Все 4 бота проекта TorLink уже работают",
		"",
		"<table bordered><tr><th>Бот</th><th>Username</th><th>Аптайм</th><th>Всего</th><th>За 24ч</th><th>Новые сегодня</th></tr>" +
			"<tr><td>TorLink</td><td>@TorLink_brobot</td><td>22ч 37м</td><td>586</td><td>46</td><td>44</td></tr></table>",
	}, "\n")
	got := ToTelegramHTML(in)
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "|") {
		t.Fatalf("остались пайпы: %s", got)
	}
}

func TestToTelegramHTMLTableKeepsInlineMarkup(t *testing.T) {
	in := strings.Join([]string{
		"**Итог** и `код`",
		"",
		"| A | B |",
		"|---|---|",
		"| *x* | a < b & c |",
		"",
		"после",
	}, "\n")
	want := strings.Join([]string{
		"<b>Итог</b> и <code>код</code>",
		"",
		"<table bordered><tr><th>A</th><th>B</th></tr><tr><td><i>x</i></td><td>a &lt; b &amp; c</td></tr></table>",
		"",
		"после",
	}, "\n")
	got := ToTelegramHTML(in)
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestToTelegramHTMLTableOnly(t *testing.T) {
	in := "| A | B |\n|---|---|\n| 1 | 2 |"
	got := ToTelegramHTML(in)
	want := "<table bordered><tr><th>A</th><th>B</th></tr><tr><td>1</td><td>2</td></tr></table>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func userProjectsReply() string {
	return strings.Join([]string{
		"ХРАЗ: а другие проекты чо",
		"Управление конструктором:",
		"",
		"Вот сводка по всем проектам:",
		"",
		"| Проект | Бот | Статус | Аптайм | Всего | За 24ч |",
		"|--------|-----|--------|--------|-------|--------|",
		"| **16. Новый бот 1** | — | ⚪ Нет токена | — | — | — |",
		"| **11. Каталог сайтов** | @testgoogglesheetsbot | 🔴 Остановлен | — | 4 | 0 |",
		"| **10. Казино** | @Jackpot_probot | 🟢 Работает | 22ч 53м | 26 | 1 |",
		"| **3. Топ обменников** | @TopExchanger_bot | 🟢 Работает | 22ч 53м | 905 | 26 |",
		"| **2. TorLink Распред** | @TorLink_navBot | 🟢 Работает | 22ч 53м | 2 440 | 59 |",
		"| **1. TorLink** | 4 бота | 🟢 Все работают | ~22ч | 3 221 | 121 |",
		"",
		"**Итого:** 8 из 9 ботов работают, 1 остановлен, у 1 проекта нет токена.",
		"",
		"Что хочешь сделать? Запустить остановленных или что-то ещё?",
	}, "\n")
}

func TestToTelegramHTMLUserProjectsReply(t *testing.T) {
	got := ToTelegramHTML(userProjectsReply())
	rich := WrapRichHTML(got)
	for _, part := range []string{
		"<p>ХРАЗ: а другие проекты чо<br>Управление конструктором:</p>",
		"<p>Вот сводка по всем проектам:</p>",
		"<table bordered><tr><th>Проект</th><th>Бот</th><th>Статус</th><th>Аптайм</th><th>Всего</th><th>За 24ч</th></tr>",
		"<td><b>16. Новый бот 1</b></td>",
		"<td>@testgoogglesheetsbot</td><td>🔴 Остановлен</td>",
		"<td>🟢 Работает</td><td>22ч 53м</td><td>2 440</td>",
		"<td>🟢 Все работают</td><td>~22ч</td><td>3 221</td>",
		"<p><b>Итого:</b> 8 из 9 ботов работают, 1 остановлен, у 1 проекта нет токена.</p>",
		"<p>Что хочешь сделать? Запустить остановленных или что-то ещё?</p>",
	} {
		if !strings.Contains(rich, part) {
			t.Fatalf("нет %q в\n%s", part, rich)
		}
	}
	if strings.Contains(rich, "**") || strings.Contains(rich, "|") {
		t.Fatalf("сырой markdown остался:\n%s", rich)
	}
}

func TestToTelegramHTMLThinSpaceInCell(t *testing.T) {
	in := "| N |\n|---|\n| 1\u202f359 |"
	got := ToTelegramHTML(in)
	if !strings.Contains(got, "<td>1\u202f359</td>") {
		t.Fatalf("тонкий пробел потерян: %s", got)
	}
	if strings.Contains(got, "|") || strings.Contains(got, "**") {
		t.Fatal(got)
	}
}

func TestPlainUserProjectsReply(t *testing.T) {
	got := Plain(userProjectsReply())
	if strings.Contains(got, "**") || strings.Contains(got, "|") {
		t.Fatalf("запасной текст всё ещё markdown:\n%s", got)
	}
	for _, part := range []string{
		"16. Новый бот 1",
		"@Jackpot_probot",
		"⚪ Нет токена",
		"2 440",
		"Итого:",
		" · ",
	} {
		if !strings.Contains(got, part) {
			t.Fatalf("нет %q в\n%s", part, got)
		}
	}
	if strings.Contains(got, "--------") {
		t.Fatal(got)
	}
}

func TestToTelegramHTMLWideTableStaysText(t *testing.T) {
	var cells []string
	for i := 0; i < 21; i++ {
		cells = append(cells, "c")
	}
	row := "| " + strings.Join(cells, " | ") + " |"
	sepCells := make([]string, 21)
	for i := range sepCells {
		sepCells[i] = "---"
	}
	sep := "| " + strings.Join(sepCells, " | ") + " |"
	in := row + "\n" + sep + "\n" + row
	got := ToTelegramHTML(in)
	if strings.Contains(got, "<table") || strings.Contains(got, "|") {
		t.Fatalf("широкая таблица не должна остаться пайпами или <table>: %s", got)
	}
	if !strings.Contains(got, " · ") {
		t.Fatalf("широкая таблица должна читаться без пайпов: %s", got)
	}
}

func TestRedactAndToken(t *testing.T) {
	in := "токен mcp_abcdef и 123456789:AAHabcdefghijklmnopqrst"
	out := Redact(in)
	if strings.Contains(out, "mcp_abcdef") || strings.Contains(out, "AAH") {
		t.Fatal(out)
	}
	if !IsMCPToken("mcp_abcdef") {
		t.Fatal("должен быть токеном")
	}
	if IsMCPToken("hello") {
		t.Fatal("ложный токен")
	}
}
