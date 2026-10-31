// @fileoverview Управление отключением автоматического подключения и его состоянием.
package telegram

import (
	"bot-builder-agent/internal/autoauth"
	"bot-builder-agent/internal/telegram/router"
	"bot-builder-agent/internal/telegram/ui"
	"context"
)

// forgetBuilder удаляет кэш подключений конструктора после смены PAT.
func (b *Bot) forgetBuilder(ctx context.Context, uid int64) {
	servers, _ := b.store.ListServers(ctx, uid)
	for _, s := range servers {
		if s.Builder && b.hub != nil {
			b.hub.Forget(uid, s.ID)
		}
	}
	if b.mcp != nil {
		b.mcp.Forget(b.cfg.BuilderURL)
	}
}

// disconnectAuto сначала сохраняет локальный запрет, затем отзывает доступ на сервере.
func (b *Bot) disconnectAuto(ctx context.Context, uid int64) error {
	automatic, err := b.store.IsAutoAccess(ctx, uid)
	if err != nil || !automatic {
		return err
	}
	if err = b.store.MarkAutoDisconnect(ctx, uid, true); err != nil {
		return err
	}
	b.forgetBuilder(ctx, uid)
	if err = b.autoClient().Disconnect(ctx, uid); err != nil {
		return err
	}
	if err = b.store.Logout(ctx, uid); err != nil {
		return err
	}
	return b.store.MarkAutoDisconnect(ctx, uid, false)
}

// autoServerMutation запрещает менять адрес/секрет автоматического сервера и обрабатывает отзыв.
func (b *Bot) autoServerMutation(uid, chat int64, res router.Result) bool {
	automatic, err := b.store.IsAutoAccess(b.baseCtx(), uid)
	if err != nil || !automatic {
		return err == nil
	}
	id := res.ServerID
	if id == 0 {
		id = b.session(uid).srvID
	}
	item, err := b.store.GetServer(b.baseCtx(), uid, id)
	if err != nil {
		return false
	}
	if !item.Builder {
		return true
	}
	switch res.Kind {
	case router.KindToggleServer, router.KindDeleteServer:
		if res.Kind == router.KindToggleServer && res.Enabled {
			b.say(chat, "Включите подключение во вкладке «Агент» конструктора.", ui.AccountKeyboard())
			return false
		}
		if err = b.disconnectAuto(b.baseCtx(), uid); err != nil {
			b.say(chat, autoauth.ErrUnavailable.Error(), ui.AccountKeyboard())
			return false
		}
		b.say(chat, "Подключение отключено. Восстановить его можно в конструкторе.", ui.AccountKeyboard())
		return false
	case router.KindServerURL, router.KindSetTransport, router.KindSetAuthKind, router.KindSaveServerSecret:
		b.say(chat, "Автоматическое подключение управляется в конструкторе. Для другого сервера добавьте отдельное подключение.", ui.AccountKeyboard())
		return false
	}
	return true
}

// autoStatus возвращает понятный тип доступа без токена и межсервисного ключа.
func (b *Bot) autoStatus(ctx context.Context, uid int64) string {
	automatic, _ := b.store.IsAutoAccess(ctx, uid)
	if automatic {
		a, _ := b.store.AutoAccess(ctx, uid)
		token, _, _ := b.store.Access(ctx, uid)
		if a.PendingDisconnect {
			return "Автоматическое: ожидается подтверждение отключения"
		}
		if token == "" {
			return "Автоматическое: отключено; восстановление во вкладке «Агент»"
		}
		if !b.cfg.AutoAuthEnabled {
			return "Автоматическое: временно выключено"
		}
		return "Автоматическое подключение Telegram"
	}
	if token, _, _ := b.store.Access(ctx, uid); token == "" {
		return "Подключение отсутствует"
	}
	return "Ручное подключение"
}
