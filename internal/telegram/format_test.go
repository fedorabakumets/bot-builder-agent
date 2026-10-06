package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"

	"bot-builder-agent/internal/telegram/ui"
)

type captureAPI struct {
	*fakeAPI
	rejectHTML    bool
	rejectRich    bool
	modes         []string
	texts         []string
	markups       []telego.ReplyMarkup
	edits         []telego.ReplyMarkup
	editMarkupErr error
	richHTML      []string
	richMD        []string
}

func (c *captureAPI) SendMessage(_ context.Context, params *telego.SendMessageParams) (*telego.Message, error) {
	if c.rejectHTML && params.ParseMode == telego.ModeHTML {
		return nil, fmt.Errorf("telego: sendMessage: %w", &telegoapi.Error{
			ErrorCode:   400,
			Description: "Bad Request: can't parse entities: Unsupported start tag",
		})
	}
	c.modes = append(c.modes, params.ParseMode)
	c.texts = append(c.texts, params.Text)
	c.markups = append(c.markups, params.ReplyMarkup)
	return &telego.Message{MessageID: len(c.texts), Chat: telego.Chat{ID: params.ChatID.ID}, Text: params.Text}, nil
}

func (c *captureAPI) SendRichMessage(_ context.Context, params *RichSendParams) (*telego.Message, error) {
	if c.rejectRich {
		return nil, fmt.Errorf("telego: sendRichMessage: %w", &telegoapi.Error{
			ErrorCode:   400,
			Description: "Bad Request: can't parse rich message",
		})
	}
	c.richHTML = append(c.richHTML, params.HTML)
	c.richMD = append(c.richMD, params.Markdown)
	text := params.HTML
	if text == "" {
		text = params.Markdown
	}
	c.modes = append(c.modes, "rich")
	c.texts = append(c.texts, text)
	c.markups = append(c.markups, params.ReplyMarkup)
	return &telego.Message{MessageID: len(c.texts), Chat: telego.Chat{ID: params.ChatID}, Text: text}, nil
}

func (c *captureAPI) EditRichMessage(_ context.Context, params *RichEditParams) (*telego.Message, error) {
	if c.rejectRich {
		return nil, fmt.Errorf("telego: editMessageText: %w", &telegoapi.Error{
			ErrorCode:   400,
			Description: "Bad Request: can't parse rich message",
		})
	}
	c.richHTML = append(c.richHTML, params.HTML)
	c.richMD = append(c.richMD, params.Markdown)
	text := params.HTML
	if text == "" {
		text = params.Markdown
	}
	c.modes = append(c.modes, "rich")
	c.texts = append(c.texts, text)
	c.markups = append(c.markups, params.ReplyMarkup)
	return &telego.Message{MessageID: params.MessageID, Text: text}, nil
}

func (c *captureAPI) EditMessageReplyMarkup(_ context.Context, params *telego.EditMessageReplyMarkupParams) (*telego.Message, error) {
	if c.editMarkupErr != nil {
		return nil, c.editMarkupErr
	}
	c.edits = append(c.edits, params.ReplyMarkup)
	return &telego.Message{MessageID: params.MessageID, Chat: telego.Chat{ID: params.ChatID.ID}}, nil
}

func (c *captureAPI) EditMessageText(_ context.Context, params *telego.EditMessageTextParams) (*telego.Message, error) {
	if c.rejectHTML && params.ParseMode == telego.ModeHTML {
		return nil, fmt.Errorf("telego: editMessageText: %w", &telegoapi.Error{
			ErrorCode:   400,
			Description: "Bad Request: can't parse entities",
		})
	}
	c.modes = append(c.modes, params.ParseMode)
	c.texts = append(c.texts, params.Text)
	c.markups = append(c.markups, params.ReplyMarkup)
	return &telego.Message{MessageID: params.MessageID, Text: params.Text}, nil
}

func TestSendAndEditUseHTML(t *testing.T) {
	b, _, _ := newTestBot(t)
	api := &captureAPI{fakeAPI: b.api.(*fakeAPI)}
	b.api = api
	kb := ui.MainKeyboard()
	msg, err := b.send(7, "**Что было:**\n* пункт про mcp_token_example", kb)
	if err != nil || msg == nil {
		t.Fatal(err)
	}
	if api.modes[0] != telego.ModeHTML {
		t.Fatalf("parse mode %q", api.modes[0])
	}
	if api.texts[0] != "<b>Что было:</b>\n* пункт про mcp_token_example" {
		t.Fatalf("text %q", api.texts[0])
	}
	if strings.Contains(api.texts[0], "**") {
		t.Fatal(api.texts[0])
	}
	if api.markups[0] == nil {
		t.Fatal("клавиатура пропала")
	}
	if err := b.edit(7, msg.MessageID, "Вызываю db_stop_bot", ui.StopKeyboard()); err != nil {
		t.Fatal(err)
	}
	if api.modes[1] != telego.ModeHTML || api.texts[1] != "Вызываю db_stop_bot" {
		t.Fatalf("edit %+v %+v", api.modes, api.texts)
	}
}

func TestSendFallsBackWhenHTMLRejected(t *testing.T) {
	b, _, _ := newTestBot(t)
	api := &captureAPI{fakeAPI: b.api.(*fakeAPI), rejectHTML: true}
	b.api = api
	body := "**Что было:**"
	msg, err := b.send(7, body, ui.MainKeyboard())
	if err != nil || msg == nil {
		t.Fatal(err)
	}
	if len(api.texts) != 1 || api.texts[0] != "Что было:" || api.modes[0] != "" {
		t.Fatalf("fallback texts=%q modes=%q", api.texts, api.modes)
	}
	if strings.Contains(api.texts[0], "**") {
		t.Fatal(api.texts[0])
	}
	if api.markups[0] == nil {
		t.Fatal("клавиатура пропала на запасном пути")
	}
	if err := b.edit(7, 1, body, nil); err != nil {
		t.Fatal(err)
	}
	if api.texts[1] != "Что было:" || api.modes[1] != "" {
		t.Fatalf("edit fallback %+v %+v", api.modes, api.texts)
	}
}

func TestSendTableUsesRichHTML(t *testing.T) {
	b, _, _ := newTestBot(t)
	api := &captureAPI{fakeAPI: b.api.(*fakeAPI)}
	b.api = api
	body := strings.Join([]string{
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
	msg, err := b.send(7, body, ui.MainKeyboard())
	if err != nil || msg == nil {
		t.Fatal(err)
	}
	if len(api.modes) != 1 || api.modes[0] != "rich" {
		t.Fatalf("modes %q", api.modes)
	}
	if api.richHTML[0] == "" || api.richMD[0] != "" {
		t.Fatalf("html %q md %q", api.richHTML, api.richMD)
	}
	html := api.richHTML[0]
	if !strings.Contains(html, "<table bordered>") || !strings.Contains(html, "<th>Проект</th>") {
		t.Fatalf("нет таблицы: %s", html)
	}
	if !strings.Contains(html, "<b>16. Новый бот 1</b>") || !strings.Contains(html, "<b>Итого:</b>") {
		t.Fatalf("жирный пропал: %s", html)
	}
	if !strings.Contains(html, "@TorLink_navBot") || !strings.Contains(html, "2 440") || !strings.Contains(html, "🟢") {
		t.Fatalf("ячейки потерялись: %s", html)
	}
	if strings.Contains(html, "**") || strings.Contains(html, "|") {
		t.Fatalf("сырой markdown в rich html: %s", html)
	}
	if api.markups[0] == nil {
		t.Fatal("клавиатура пропала")
	}
}

func TestSendTableFallsBackWhenRichRejected(t *testing.T) {
	b, _, _ := newTestBot(t)
	api := &captureAPI{fakeAPI: b.api.(*fakeAPI), rejectRich: true}
	b.api = api
	body := strings.Join([]string{
		"| Бот | Username |",
		"|-----|----------|",
		"| **TorLink** | @TorLink_brobot |",
	}, "\n")
	msg, err := b.send(7, body, ui.MainKeyboard())
	if err != nil || msg == nil {
		t.Fatal(err)
	}
	if len(api.texts) != 1 || api.modes[0] != "" {
		t.Fatalf("fallback texts=%q modes=%q", api.texts, api.modes)
	}
	if strings.Contains(api.texts[0], "**") || strings.Contains(api.texts[0], "|") {
		t.Fatalf("запасной текст сырой: %q", api.texts[0])
	}
	if !strings.Contains(api.texts[0], "TorLink") || !strings.Contains(api.texts[0], "@TorLink_brobot") || !strings.Contains(api.texts[0], " · ") {
		t.Fatalf("нечитаемый запасной текст: %q", api.texts[0])
	}
	if api.markups[0] == nil {
		t.Fatal("клавиатура пропала")
	}
}

func TestHTMLRejectedDetectsParseError(t *testing.T) {
	err := fmt.Errorf("wrap: %w", &telegoapi.Error{ErrorCode: 400, Description: "Bad Request: can't parse entities"})
	if !htmlRejected(err) {
		t.Fatal("parse error must retry")
	}
	if !htmlRejected(errors.New("Bad Request: message is too long")) {
		t.Fatal("too long must retry")
	}
	if htmlRejected(errors.New("bot was blocked by the user")) {
		t.Fatal("чужая ошибка не должна маскироваться повторной отправкой")
	}
}
