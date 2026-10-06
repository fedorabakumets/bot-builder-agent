package ui

import (
	"testing"

	"github.com/mymmrac/telego"
)

func TestReplyLabels(t *testing.T) {
	screens := []struct {
		name string
		kb   *telego.ReplyKeyboardMarkup
		need []string
	}{
		{"main", MainKeyboard(), []string{BtnTask, BtnProjects, BtnBots, BtnAccount}},
		{"account", AccountKeyboard(), []string{BtnToken, BtnStatus, BtnReset, BtnLogout, BtnBack}},
		{"projects", ProjectsKeyboard(), []string{BtnCreate, BtnActive, BtnArchive, BtnBack}},
		{"bots", BotsKeyboard(), []string{BtnConnect, BtnBack}},
		{"wait", WaitKeyboard(), []string{BtnCancel}},
	}
	for _, sc := range screens {
		got := labels(sc.kb)
		for _, need := range sc.need {
			if !has(got, need) {
				t.Fatalf("%s нет %q в %v", sc.name, need, got)
			}
		}
		if !sc.kb.ResizeKeyboard {
			t.Fatalf("%s без ResizeKeyboard", sc.name)
		}
	}
	if !MenuText(BtnBack) || MenuText("сделай бота") {
		t.Fatal("MenuText")
	}
}

func TestInlineCallbacksFit(t *testing.T) {
	projects := []Project{{ID: 10, Name: "Первый"}, {ID: 11, Name: "Второй"}}
	bots := []BotToken{{ID: 7, Name: "shop", Username: "shopbot"}}
	boards := []*telego.InlineKeyboardMarkup{
		ProjectListKeyboard(projects, 0, false),
		ProjectListKeyboard(makeProjects(25), 1, true),
		ProjectCardKeyboard(99),
		BotListKeyboard(3, bots),
		BotCardKeyboard(3, 7),
		ConfirmKeyboard("yes:logout", "no:logout"),
		ConfirmKeyboard("yes:pdel:99", "no:pdel:99"),
		ConfirmKeyboard("yes:stop:3:7", "no:stop:3:7"),
		ConfirmKeyboard("yes:bdel:3:7", "no:bdel:3:7"),
		ConfirmKeyboard("yes:agent", "no:agent"),
		StopKeyboard(),
	}
	for _, kb := range boards {
		for _, data := range CallbacksOf(kb) {
			if len(data) == 0 || len(data) > 64 {
				t.Fatalf("callback %q len %d", data, len(data))
			}
			if _, ok := ParseCallback(data); !ok {
				t.Fatalf("не разобран %q", data)
			}
		}
	}
	cb, ok := ParseCallback("b:ret:3:7:30")
	if !ok || cb.Name != "retention" || cb.Days != 30 || cb.ProjectID != 3 || cb.TokenID != 7 {
		t.Fatalf("%+v %v", cb, ok)
	}
	if _, ok := ParseCallback("b:ret:3:7:5"); ok {
		t.Fatal("срок 5 дней нельзя")
	}
	page, ok := ParseCallback("p:alist:2")
	if !ok || !page.Archived || page.Page != 2 {
		t.Fatalf("%+v", page)
	}
}

func makeProjects(n int) []Project {
	out := make([]Project, n)
	for i := range out {
		out[i] = Project{ID: int64(i + 1), Name: "p"}
	}
	return out
}

func labels(kb *telego.ReplyKeyboardMarkup) []string {
	var out []string
	for _, row := range kb.Keyboard {
		for _, btn := range row {
			out = append(out, btn.Text)
		}
	}
	return out
}

func has(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
