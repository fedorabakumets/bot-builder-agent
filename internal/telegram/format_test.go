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
	rejectHTML bool
	modes      []string
	texts      []string
	markups    []telego.ReplyMarkup
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
	if len(api.texts) != 1 || api.texts[0] != body || api.modes[0] != "" {
		t.Fatalf("fallback texts=%q modes=%q", api.texts, api.modes)
	}
	if api.markups[0] == nil {
		t.Fatal("клавиатура пропала на запасном пути")
	}
	if err := b.edit(7, 1, body, nil); err != nil {
		t.Fatal(err)
	}
	if api.texts[1] != body || api.modes[1] != "" {
		t.Fatalf("edit fallback %+v %+v", api.modes, api.texts)
	}
}

func TestSendTableFallsBackWhenHTMLRejected(t *testing.T) {
	b, _, _ := newTestBot(t)
	api := &captureAPI{fakeAPI: b.api.(*fakeAPI), rejectHTML: true}
	b.api = api
	body := strings.Join([]string{
		"| Бот | Username |",
		"|-----|----------|",
		"| TorLink | @TorLink_brobot |",
	}, "\n")
	msg, err := b.send(7, body, ui.MainKeyboard())
	if err != nil || msg == nil {
		t.Fatal(err)
	}
	if len(api.texts) != 1 || api.texts[0] != body || api.modes[0] != "" {
		t.Fatalf("fallback texts=%q modes=%q", api.texts, api.modes)
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
