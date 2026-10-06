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
		MenuButton("main"),
		MenuButton("account"),
		ScreenActions("main"),
		ScreenActions("account"),
		ScreenActions("projects"),
		ScreenActions("bots"),
		ScreenActions("wait"),
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

func TestMenuCallbackReturnsMainActions(t *testing.T) {
	kb, ok := ActionsForMenu("menu")
	if !ok || kb == nil {
		t.Fatal("menu")
	}
	got := inlineLabels(kb)
	for _, need := range []string{BtnTask, BtnProjects, BtnBots, BtnAccount} {
		if !has(got, need) {
			t.Fatalf("нет %q в %v", need, got)
		}
	}
	if len(got) != 4 {
		t.Fatalf("главное меню: %v", got)
	}
	for _, data := range CallbacksOf(kb) {
		cb, ok := ParseCallback(data)
		if !ok || cb.Name == "" {
			t.Fatalf("кнопка меню %q", data)
		}
	}
	account, ok := ActionsForMenu("menu:account")
	if !ok {
		t.Fatal("menu:account")
	}
	acc := inlineLabels(account)
	for _, need := range []string{BtnToken, BtnStatus, BtnReset, BtnLogout, BtnBack} {
		if !has(acc, need) {
			t.Fatalf("аккаунт без %q: %v", need, acc)
		}
	}
	projects, ok := ActionsForMenu("menu:projects")
	if !ok || !has(inlineLabels(projects), BtnCreate) || !has(inlineLabels(projects), BtnArchive) {
		t.Fatal("проекты")
	}
	bots, ok := ActionsForMenu("menu:bots")
	if !ok || !has(inlineLabels(bots), BtnConnect) || !has(inlineLabels(bots), BtnBack) {
		t.Fatal("боты")
	}
	if _, ok := ActionsForMenu("nav:task"); ok {
		t.Fatal("nav — это не открытие меню")
	}
	if _, ok := ParseCallback("menu:nope"); ok {
		t.Fatal("чужой экран")
	}
}

func inlineLabels(kb *telego.InlineKeyboardMarkup) []string {
	var out []string
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			out = append(out, button.Text)
		}
	}
	return out
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
