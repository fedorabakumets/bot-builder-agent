package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mymmrac/telego"

	"bot-builder-agent/internal/telegram/ui"
)

func TestGroupChatDoesNotGetReplyKeyboard(t *testing.T) {
	b, raw, _ := newTestBot(t)
	api := &captureAPI{fakeAPI: raw}
	b.api = api

	screens := []telego.ReplyMarkup{
		ui.MainKeyboard(),
		ui.AccountKeyboard(),
		ui.ProjectsKeyboard(),
		ui.BotsKeyboard(),
		ui.WaitKeyboard(),
		ui.ConfirmKeyboard("yes:logout", "no:logout"),
		ui.StopKeyboard(),
	}
	for i, kb := range screens {
		msg, err := b.send(-100, "ответ", kb)
		if err != nil || msg == nil {
			t.Fatalf("экран %d: %v", i, err)
		}
	}
	content := inlineMarkups(t, api.markups)
	if len(content) != len(screens) {
		t.Fatalf("сообщений с кнопками %d", len(content))
	}
	if len(api.edits) != 0 {
		t.Fatalf("кнопки нельзя вешать правкой: %d", len(api.edits))
	}
	menu := content[0]
	if len(menu.InlineKeyboard) != 1 || menu.InlineKeyboard[0][0].Text != ui.BtnMenu {
		t.Fatalf("главное меню: %+v", menu)
	}
	if menu.InlineKeyboard[0][0].CallbackData != "menu" {
		t.Fatalf("callback %q", menu.InlineKeyboard[0][0].CallbackData)
	}
	opened, ok := ui.ActionsForMenu(menu.InlineKeyboard[0][0].CallbackData)
	if !ok {
		t.Fatal("кнопка меню не открывает действия")
	}
	labels := map[string]bool{}
	for _, row := range opened.InlineKeyboard {
		for _, button := range row {
			labels[button.Text] = true
		}
	}
	for _, need := range []string{ui.BtnTask, ui.BtnProjects, ui.BtnBots, ui.BtnAccount} {
		if !labels[need] {
			t.Fatalf("нет %q", need)
		}
	}
	if content[1].InlineKeyboard[0][0].CallbackData != "menu:account" {
		t.Fatalf("аккаунт: %+v", content[1])
	}
	confirm := content[5]
	if !strings.Contains(confirm.InlineKeyboard[0][0].CallbackData, "yes:logout") {
		t.Fatalf("подтверждение без Да: %+v", confirm)
	}
}

func TestPrivateChatDoesNotGetReplyKeyboard(t *testing.T) {
	b, raw, _ := newTestBot(t)
	api := &captureAPI{fakeAPI: raw}
	b.api = api

	screens := []telego.ReplyMarkup{
		ui.MainKeyboard(),
		ui.AccountKeyboard(),
		ui.ProjectsKeyboard(),
		ui.BotsKeyboard(),
		ui.WaitKeyboard(),
		ui.ConfirmKeyboard("yes:logout", "no:logout"),
		ui.StopKeyboard(),
	}
	for i, kb := range screens {
		msg, err := b.send(7, "ответ", kb)
		if err != nil || msg == nil {
			t.Fatalf("экран %d: %v", i, err)
		}
	}
	content := inlineMarkups(t, api.markups)
	if len(content) != len(screens) {
		t.Fatalf("сообщений с кнопками %d", len(content))
	}
	if len(api.edits) != 0 {
		t.Fatalf("кнопки нельзя вешать правкой: %d", len(api.edits))
	}
	menu := content[0]
	if len(menu.InlineKeyboard) != 1 || menu.InlineKeyboard[0][0].Text != ui.BtnMenu {
		t.Fatalf("главное меню: %+v", menu)
	}
	if menu.InlineKeyboard[0][0].CallbackData != "menu" {
		t.Fatalf("callback %q", menu.InlineKeyboard[0][0].CallbackData)
	}
	want := []string{"menu", "menu:account", "menu:projects", "menu:bots", "menu:wait"}
	for i, data := range want {
		if content[i].InlineKeyboard[0][0].CallbackData != data {
			t.Fatalf("экран %d: %+v", i, content[i])
		}
	}
	if !strings.Contains(content[5].InlineKeyboard[0][0].CallbackData, "yes:logout") {
		t.Fatalf("подтверждение без Да: %+v", content[5])
	}
	if content[6].InlineKeyboard[0][0].CallbackData != "run:stop" {
		t.Fatalf("стоп без кнопки: %+v", content[6])
	}
}

func TestGroupMenuCallbackShowsMainActions(t *testing.T) {
	b, raw, _ := newTestBot(t)
	api := &captureAPI{fakeAPI: raw}
	b.api = api
	msg := &telego.Message{MessageID: 55, Chat: telego.Chat{ID: -100}}
	b.process(telego.Update{CallbackQuery: &telego.CallbackQuery{
		ID:      "cb-menu",
		From:    telego.User{ID: 9},
		Data:    "menu",
		Message: msg,
	}})
	if len(api.markups) != 0 {
		t.Fatalf("меню не должно слать новый пузырь: %+v", api.markups)
	}
	if len(api.edits) != 1 {
		t.Fatalf("правок %d", len(api.edits))
	}
	kb, ok := api.edits[0].(*telego.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("%T", api.edits[0])
	}
	got := map[string]string{}
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			got[button.Text] = button.CallbackData
		}
	}
	for _, need := range []string{ui.BtnTask, ui.BtnProjects, ui.BtnBots, ui.BtnAccount} {
		if got[need] == "" {
			t.Fatalf("нет %q в %+v", need, got)
		}
	}
	task := pressMenu(b, api, got[ui.BtnTask])
	if task == "" || !strings.Contains(strings.ToLower(task), "упомян") {
		t.Fatalf("текст задачи: %q", task)
	}
}

func TestGroupSendKeepsAnswerWhenMenuEditFails(t *testing.T) {
	b, raw, _ := newTestBot(t)
	api := &captureAPI{fakeAPI: raw, editMarkupErr: errors.New("edit failed")}
	b.api = api
	msg, err := b.send(-77, "ответ бота", ui.MainKeyboard())
	if err != nil || msg == nil || msg.Text == "" {
		t.Fatalf("ответ должен уйти: %v %+v", err, msg)
	}
	kb, ok := api.markups[0].(*telego.InlineKeyboardMarkup)
	if !ok || kb.InlineKeyboard[0][0].CallbackData != "menu" {
		t.Fatalf("меню должно быть на самом сообщении: %T %+v", api.markups[0], api.markups[0])
	}
	if len(api.edits) != 0 {
		t.Fatal("кнопки не должны вешаться правкой")
	}
}

func TestGroupStartGoesThroughSend(t *testing.T) {
	b, raw, _ := newTestBot(t)
	api := &captureAPI{fakeAPI: raw}
	b.api = api
	b.process(telego.Update{Message: &telego.Message{
		From: &telego.User{ID: 9},
		Chat: telego.Chat{ID: -100123, Type: telego.ChatTypeSupergroup},
		Text: "/start",
	}})
	if len(api.markups) == 0 {
		t.Fatal("нет ответа")
	}
	for _, markup := range api.markups {
		if _, ok := markup.(*telego.ReplyKeyboardMarkup); ok {
			t.Fatal("группа получила reply-клавиатуру")
		}
	}
	before := len(api.markups)
	b.process(textUpdate(9, "/start"))
	if len(api.markups) == before {
		t.Fatal("личка без ответа")
	}
	var menu *telego.InlineKeyboardMarkup
	for _, markup := range api.markups[before:] {
		if _, ok := markup.(*telego.ReplyKeyboardMarkup); ok {
			t.Fatal("личка получила reply-клавиатуру")
		}
		if kb, ok := markup.(*telego.InlineKeyboardMarkup); ok {
			menu = kb
		}
	}
	if menu == nil || menu.InlineKeyboard[0][0].Text != ui.BtnMenu || menu.InlineKeyboard[0][0].CallbackData != "menu" {
		t.Fatalf("меню лички: %+v", menu)
	}
}

func TestPrivateMenuTaskDoesNotAskForMention(t *testing.T) {
	b, raw, _ := newTestBot(t)
	api := &captureAPI{fakeAPI: raw}
	b.api = api
	msg := &telego.Message{MessageID: 8, Chat: telego.Chat{ID: 7, Type: telego.ChatTypePrivate}}
	b.process(telego.Update{CallbackQuery: &telego.CallbackQuery{
		ID:      "cb-menu",
		From:    telego.User{ID: 7},
		Data:    "menu",
		Message: msg,
	}})
	if len(api.edits) != 1 {
		t.Fatalf("правок %d", len(api.edits))
	}
	kb, ok := api.edits[0].(*telego.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("%T", api.edits[0])
	}
	var taskData string
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			if button.Text == ui.BtnTask {
				taskData = button.CallbackData
			}
		}
	}
	if taskData == "" {
		t.Fatal("нет задачи")
	}
	before := len(api.texts)
	b.process(telego.Update{CallbackQuery: &telego.CallbackQuery{
		ID:   "cb-task",
		From: telego.User{ID: 7},
		Data: taskData,
		Message: &telego.Message{
			MessageID: 8,
			Chat:      telego.Chat{ID: 7, Type: telego.ChatTypePrivate},
		},
	}})
	if len(api.texts) == before {
		t.Fatal("нет подсказки")
	}
	task := lastVisible(api.texts)
	low := strings.ToLower(task)
	if strings.Contains(low, "упомян") || strings.Contains(low, "ответ") {
		t.Fatalf("личке нельзя про упоминание: %q", task)
	}
	if !strings.Contains(low, "опишите") {
		t.Fatalf("текст задачи: %q", task)
	}
}

func TestLogoutConfirmButtonsStayOnTheMessage(t *testing.T) {
	b, raw, _ := newTestBot(t)
	api := &captureAPI{fakeAPI: raw}
	b.api = api
	if err := b.store.SaveToken(context.Background(), 7, "mcp_user_7_xx"); err != nil {
		t.Fatal(err)
	}
	b.process(textUpdate(7, "/logout"))
	var confirm *telego.InlineKeyboardMarkup
	for i, text := range api.texts {
		if text != "Удалить сохранённый токен?" {
			continue
		}
		kb, ok := api.markups[i].(*telego.InlineKeyboardMarkup)
		if !ok {
			t.Fatalf("вопрос без кнопок: %T", api.markups[i])
		}
		confirm = kb
	}
	if confirm == nil {
		t.Fatal("нет вопроса")
	}
	got := map[string]string{}
	for _, row := range confirm.InlineKeyboard {
		for _, button := range row {
			got[button.Text] = button.CallbackData
		}
	}
	if got[ui.BtnYes] != "yes:logout" || got[ui.BtnNo] != "no:logout" {
		t.Fatalf("кнопки %+v", got)
	}
	if len(api.edits) != 0 {
		t.Fatal("подтверждение не должно зависеть от правки")
	}
	b.process(callbackUpdate(7, "yes:logout"))
	if lastVisible(api.texts) != "Токен удалён." {
		t.Fatalf("после Да: %+v", api.texts)
	}
	token, err := b.store.Token(context.Background(), 7)
	if err != nil || token != "" {
		t.Fatalf("токен %q err %v", token, err)
	}
}

func pressMenu(b *Bot, api *captureAPI, data string) string {
	before := len(api.texts)
	b.process(telego.Update{CallbackQuery: &telego.CallbackQuery{
		ID:   "cb-" + data,
		From: telego.User{ID: 9},
		Data: data,
		Message: &telego.Message{
			MessageID: 55,
			Chat:      telego.Chat{ID: -100},
		},
	}})
	if len(api.texts) == before {
		return ""
	}
	return lastVisible(api.texts[before:])
}

func lastVisible(texts []string) string {
	for i := len(texts) - 1; i >= 0; i-- {
		if texts[i] != replyHideText {
			return texts[i]
		}
	}
	return ""
}

func inlineMarkups(t *testing.T, markups []telego.ReplyMarkup) []*telego.InlineKeyboardMarkup {
	t.Helper()
	var out []*telego.InlineKeyboardMarkup
	removed := 0
	for i, markup := range markups {
		switch m := markup.(type) {
		case *telego.ReplyKeyboardMarkup:
			t.Fatalf("отправка %d: ReplyKeyboardMarkup", i)
		case *telego.ReplyKeyboardRemove:
			if !m.RemoveKeyboard {
				t.Fatalf("отправка %d: снятие без флага", i)
			}
			removed++
		case *telego.InlineKeyboardMarkup:
			out = append(out, m)
		default:
			t.Fatalf("отправка %d: %T", i, markup)
		}
	}
	if removed != 1 {
		t.Fatalf("снятий клавиатуры %d", removed)
	}
	return out
}
