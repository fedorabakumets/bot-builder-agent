package telegram

import (
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
	if len(api.markups) != len(screens) {
		t.Fatalf("отправок %d", len(api.markups))
	}
	for i, markup := range api.markups {
		if _, ok := markup.(*telego.ReplyKeyboardMarkup); ok {
			t.Fatalf("отправка %d: ReplyKeyboardMarkup", i)
		}
		removed, ok := markup.(*telego.ReplyKeyboardRemove)
		if !ok || !removed.RemoveKeyboard {
			t.Fatalf("отправка %d: %T", i, markup)
		}
	}
	if len(api.edits) != len(screens) {
		t.Fatalf("правок меню %d", len(api.edits))
	}
	menu, ok := api.edits[0].(*telego.InlineKeyboardMarkup)
	if !ok || len(menu.InlineKeyboard) != 1 || menu.InlineKeyboard[0][0].Text != ui.BtnMenu {
		t.Fatalf("главное меню: %+v", api.edits[0])
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
	account, ok := api.edits[1].(*telego.InlineKeyboardMarkup)
	if !ok || account.InlineKeyboard[0][0].CallbackData != "menu:account" {
		t.Fatalf("аккаунт: %+v", api.edits[1])
	}
	confirm, ok := api.edits[5].(*telego.InlineKeyboardMarkup)
	if !ok || !strings.Contains(confirm.InlineKeyboard[0][0].CallbackData, "yes:logout") {
		t.Fatalf("подтверждение не осталось inline: %+v", api.edits[5])
	}
}

func TestPrivateChatKeepsReplyKeyboard(t *testing.T) {
	b, raw, _ := newTestBot(t)
	api := &captureAPI{fakeAPI: raw}
	b.api = api
	msg, err := b.send(7, "ответ", ui.MainKeyboard())
	if err != nil || msg == nil {
		t.Fatal(err)
	}
	kb, ok := api.markups[0].(*telego.ReplyKeyboardMarkup)
	if !ok || !kb.IsPersistent || !kb.ResizeKeyboard {
		t.Fatalf("личка: %T %+v", api.markups[0], api.markups[0])
	}
	if len(api.edits) != 0 {
		t.Fatal("в личке не нужна правка меню")
	}
	if _, err := b.send(7, "ещё", ui.AccountKeyboard()); err != nil {
		t.Fatal(err)
	}
	if _, ok := api.markups[1].(*telego.ReplyKeyboardMarkup); !ok {
		t.Fatalf("аккаунт в личке: %T", api.markups[1])
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
	if _, ok := api.markups[0].(*telego.ReplyKeyboardRemove); !ok {
		t.Fatalf("клавиатура не снята: %T", api.markups[0])
	}
	if len(api.edits) != 0 {
		t.Fatal("правка не должна записаться после ошибки")
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
	b.process(textUpdate(9, "/start"))
	var private *telego.ReplyKeyboardMarkup
	for _, markup := range api.markups {
		if kb, ok := markup.(*telego.ReplyKeyboardMarkup); ok {
			private = kb
		}
	}
	if private == nil || !private.IsPersistent {
		t.Fatal("личка потеряла reply-клавиатуру")
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
	return api.texts[len(api.texts)-1]
}
