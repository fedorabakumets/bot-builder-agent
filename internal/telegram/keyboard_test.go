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
	want := []string{"menu", "menu:account", "menu:projects", "menu:bots", "menu:wait"}
	for i, data := range want {
		kb, ok := api.edits[i].(*telego.InlineKeyboardMarkup)
		if !ok || kb.InlineKeyboard[0][0].CallbackData != data {
			t.Fatalf("экран %d: %+v", i, api.edits[i])
		}
	}
	confirm, ok := api.edits[5].(*telego.InlineKeyboardMarkup)
	if !ok || !strings.Contains(confirm.InlineKeyboard[0][0].CallbackData, "yes:logout") {
		t.Fatalf("подтверждение не осталось inline: %+v", api.edits[5])
	}
	stop, ok := api.edits[6].(*telego.InlineKeyboardMarkup)
	if !ok || stop.InlineKeyboard[0][0].CallbackData != "run:stop" {
		t.Fatalf("стоп не остался inline: %+v", api.edits[6])
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
	before := len(api.markups)
	b.process(textUpdate(9, "/start"))
	if len(api.markups) == before {
		t.Fatal("личка без ответа")
	}
	for _, markup := range api.markups[before:] {
		if _, ok := markup.(*telego.ReplyKeyboardMarkup); ok {
			t.Fatal("личка получила reply-клавиатуру")
		}
		if _, ok := markup.(*telego.ReplyKeyboardRemove); !ok {
			t.Fatalf("личка: %T", markup)
		}
	}
	if len(api.edits) == 0 {
		t.Fatal("личка без кнопки меню")
	}
	last := api.edits[len(api.edits)-1]
	menu, ok := last.(*telego.InlineKeyboardMarkup)
	if !ok || menu.InlineKeyboard[0][0].Text != ui.BtnMenu || menu.InlineKeyboard[0][0].CallbackData != "menu" {
		t.Fatalf("меню лички: %+v", last)
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
	task := api.texts[len(api.texts)-1]
	low := strings.ToLower(task)
	if strings.Contains(low, "упомян") || strings.Contains(low, "ответ") {
		t.Fatalf("личке нельзя про упоминание: %q", task)
	}
	if !strings.Contains(low, "опишите") {
		t.Fatalf("текст задачи: %q", task)
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
