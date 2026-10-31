// @fileoverview Автоматический доступ для подтверждённых отправителей в личных и групповых чатах.
package telegram

import (
	"bot-builder-agent/internal/autoauth"
	"bot-builder-agent/internal/mcp"
	"bot-builder-agent/internal/telegram/router"
	"bot-builder-agent/internal/telegram/ui"
	"context"
	"errors"
	"github.com/mymmrac/telego"
	"strings"
	"time"
)

// autoClient использует только конфигурацию оператора, никогда пользовательский MCP URL.
func (b *Bot) autoClient() autoauth.Client {
	return autoauth.Client{Endpoint: b.cfg.BuilderURL, AgentID: b.cfg.AutoAuthID, Secret: b.cfg.AutoAuthSecret}
}

// prepareAutoAccess вызывается под блокировкой пользователя до маршрутизации команды.
func (b *Bot) prepareAutoAccess(upd telego.Update, uid, chat int64, sess *session) bool {
	isAuto, err := b.store.IsAutoAccess(b.baseCtx(), uid)
	if err != nil {
		b.say(chat, "Не удалось прочитать подключение.", ui.AccountKeyboard())
		return false
	}
	token, _, err := b.store.Access(b.baseCtx(), uid)
	if err != nil {
		return false
	}
	// Ручной доступ имеет приоритет; остановка задачи не ждёт обновления подключения.
	if token != "" && !isAuto || sess.busy {
		return true
	}
	raw := ""
	callback := ""
	if upd.Message != nil {
		raw = strings.TrimSpace(upd.Message.Text)
	}
	if upd.CallbackQuery != nil {
		callback = upd.CallbackQuery.Data
	}
	// Ручной импорт остаётся доступным даже после отзыва автоматического подключения.
	if _, ok := mcp.ParseServers(raw); ok {
		return true
	}
	// Отключение, статус и ручной ввод доступны даже при недоступности сервера.
	if raw == "/logout" || raw == "/token" || strings.HasPrefix(raw, "/token ") || raw == "/status" || raw == "/cancel" ||
		raw == ui.BtnLogout || raw == ui.BtnToken || raw == ui.BtnAccount ||
		callback == "yes:logout" || callback == "no:logout" || strings.HasPrefix(callback, "yes:sdel:") ||
		strings.HasPrefix(callback, "s:off:") || strings.HasPrefix(callback, "s:del:") || callback == "s:list" || callback == "nav:servers" || raw == "/servers" || sess.state == router.StateWaitMCP {
		return true
	}
	if !b.cfg.AutoAuthEnabled {
		if isAuto {
			b.say(chat, "Автоматическое подключение временно выключено.", ui.AccountKeyboard())
			return false
		}
		return true
	}
	if !b.cfg.Allowed(uid) {
		return true
	}
	var user *telego.User
	if upd.CallbackQuery != nil {
		user = &upd.CallbackQuery.From
	} else if upd.Message != nil {
		user = upd.Message.From
	}
	if user == nil || user.ID != uid || user.IsBot {
		return false
	}
	ctx, cancel := context.WithTimeout(b.baseCtx(), 20*time.Second)
	defer cancel()
	a, err := b.store.AutoAccess(ctx, uid)
	if err == nil && a.PendingDisconnect {
		err = b.autoClient().Disconnect(ctx, uid)
		if err == nil {
			err = b.store.Logout(ctx, uid)
		}
		if err == nil {
			err = b.store.MarkAutoDisconnect(ctx, uid, false)
		}
		token = ""
	}
	if err == nil && token != "" && a.ExpiresAt.After(time.Now().Add(5*time.Minute)) {
		return true
	}
	var access autoauth.Access
	if err == nil {
		access, err = b.autoClient().Exchange(ctx, autoauth.Identity{ID: uid, FirstName: user.FirstName, LastName: user.LastName, Username: user.Username})
	}
	if err == nil {
		err = b.store.SaveAutoAccess(ctx, uid, b.cfg.BuilderURL, access)
		if err == nil {
			b.forgetBuilder(ctx, uid)
			return true
		}
	}
	message := autoauth.ErrUnavailable.Error()
	if errors.Is(err, autoauth.ErrRevoked) {
		message = autoauth.ErrRevoked.Error()
	}
	b.say(chat, message, ui.AccountKeyboard())
	return false
}
