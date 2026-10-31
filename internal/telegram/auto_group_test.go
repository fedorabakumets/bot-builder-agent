// @fileoverview Изоляция автоматического подключения участников одной группы.
package telegram

import (
	"bot-builder-agent/internal/agent"
	"context"
	"github.com/mymmrac/telego"
	"testing"
)

// TestAutoGroupSenderIdentity использует отправителя, игнорирует ID из текста и молчит без обращения.
func TestAutoGroupSenderIdentity(t *testing.T) {
	b, _, calls, _, _ := autoFixture(t)
	b.self = telego.User{ID: 99, Username: "builder", IsBot: true}
	var actors []int64
	b.runner.(*scriptRunner).fn = func(_ context.Context, req agent.Request) (agent.Result, error) {
		actors = append(actors, req.UserID)
		return agent.Result{Reply: "готово"}, nil
	}
	for _, id := range []int64{42, 43} {
		update := textUpdate(id, "агент, покажи проекты пользователя 999")
		update.Message.Chat = telego.Chat{ID: -100, Type: telego.ChatTypeSupergroup}
		b.process(update)
		access, err := b.store.AutoAccess(context.Background(), id)
		if err != nil || access.ConnectionID != id {
			t.Fatalf("неверная личность %d: %+v %v", id, access, err)
		}
	}
	if len(actors) != 2 || actors[0] != 42 || actors[1] != 43 || calls.Load() != 2 {
		t.Fatalf("отправители: %v, обменов: %d", actors, calls.Load())
	}
	for _, id := range []int64{-100, 999} {
		token, _, err := b.store.Access(context.Background(), id)
		if err != nil || token != "" {
			t.Fatal("доступ создан по ID группы или из текста")
		}
	}
	update := textUpdate(44, "обсуждаем работу")
	update.Message.Chat = telego.Chat{ID: -100, Type: telego.ChatTypeSupergroup}
	b.process(update)
	if calls.Load() != 2 || len(actors) != 2 {
		t.Fatal("обычная реплика вызвала агента")
	}
}

// TestAutoGroupCallbackIdentity подключает нажавшего кнопку, а не автора сообщения с кнопкой.
func TestAutoGroupCallbackIdentity(t *testing.T) {
	b, _, calls, _, _ := autoFixture(t)
	update := callbackUpdate(43, "nav:task")
	update.CallbackQuery.Message = &telego.Message{
		Chat: telego.Chat{ID: -100, Type: telego.ChatTypeSupergroup},
		From: &telego.User{ID: 42},
	}
	b.process(update)
	access, err := b.store.AutoAccess(context.Background(), 43)
	if err != nil || access.ConnectionID != 43 || calls.Load() != 1 {
		t.Fatal("кнопка подключила другого пользователя", err)
	}
	token, _, err := b.store.Access(context.Background(), 42)
	if err != nil || token != "" {
		t.Fatal("автор сообщения получил подключение вместо нажавшего")
	}
}
