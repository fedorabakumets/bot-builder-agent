package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
)

// richTextLimit — предел Bot API для rich message: 32768 UTF-8 символов.
const richTextLimit = 32768

// RichSendParams — sendRichMessage. Ровно одно из HTML и Markdown
// уходит в InputRichMessage (https://core.telegram.org/bots/api#inputrichmessage).
type RichSendParams struct {
	ChatID      int64
	HTML        string
	Markdown    string
	ReplyMarkup telego.ReplyMarkup
}

// RichEditParams — editMessageText с полем rich_message вместо text.
type RichEditParams struct {
	ChatID      int64
	MessageID   int
	HTML        string
	Markdown    string
	ReplyMarkup *telego.InlineKeyboardMarkup
}

type liveAPI struct {
	*telego.Bot
	httpClient *http.Client
}

func newLiveAPI(token string) (*liveAPI, error) {
	api, err := telego.NewBot(token, telego.WithDefaultLogger(false, false))
	if err != nil {
		return nil, err
	}
	return &liveAPI{
		Bot:        api,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (a *liveAPI) SendRichMessage(ctx context.Context, p *RichSendParams) (*telego.Message, error) {
	payload := map[string]any{
		"chat_id":      p.ChatID,
		"rich_message": richDocument(p.HTML, p.Markdown),
	}
	if p.ReplyMarkup != nil {
		payload["reply_markup"] = p.ReplyMarkup
	}
	return a.call(ctx, "sendRichMessage", payload)
}

func (a *liveAPI) EditRichMessage(ctx context.Context, p *RichEditParams) (*telego.Message, error) {
	payload := map[string]any{
		"chat_id":      p.ChatID,
		"message_id":   p.MessageID,
		"rich_message": richDocument(p.HTML, p.Markdown),
	}
	if p.ReplyMarkup != nil {
		payload["reply_markup"] = p.ReplyMarkup
	}
	return a.call(ctx, "editMessageText", payload)
}

func richDocument(html, markdown string) map[string]string {
	if strings.TrimSpace(html) != "" {
		return map[string]string{"html": html}
	}
	return map[string]string{"markdown": markdown}
}

func (a *liveAPI) call(ctx context.Context, method string, payload any) (*telego.Message, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.methodURL(method), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram %s: сеть", method)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("telegram %s: чтение ответа: %w", method, err)
	}
	var parsed struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		ErrorCode   int             `json:"error_code"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("telegram %s: ответ не json", method)
	}
	if !parsed.OK {
		return nil, &telegoapi.Error{ErrorCode: parsed.ErrorCode, Description: parsed.Description}
	}
	var msg telego.Message
	if len(parsed.Result) > 0 && parsed.Result[0] == '{' {
		if err := json.Unmarshal(parsed.Result, &msg); err != nil {
			return nil, fmt.Errorf("telegram %s: сообщение: %w", method, err)
		}
	}
	return &msg, nil
}

func (a *liveAPI) methodURL(method string) string {
	return "https://api.telegram.org/bot" + a.Token() + "/" + method
}
