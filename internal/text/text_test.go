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
