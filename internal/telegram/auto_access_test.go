// @fileoverview Проверка отправителя, обновления, отключения и запрета доступа из групп.
package telegram

import (
	"bot-builder-agent/internal/agent"
	"bot-builder-agent/internal/autoauth"
	"bot-builder-agent/internal/telegram/router"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// autoFixture создаёт доверенный сервер с управляемым состоянием отзыва.
func autoFixture(t *testing.T) (*Bot, *fakeAPI, *atomic.Int32, *atomic.Bool, *atomic.Bool) {
	t.Helper()
	b, api, run := newTestBot(t)
	var calls atomic.Int32
	var revoked, down atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if down.Load() {
			w.WriteHeader(503)
			return
		}
		if strings.HasSuffix(r.URL.Path, "disconnect") {
			revoked.Store(true)
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		}
		calls.Add(1)
		if revoked.Load() {
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"error":"CONNECTION_REVOKED"}`))
			return
		}
		var user autoauth.Identity
		_ = json.NewDecoder(r.Body).Decode(&user)
		_ = json.NewEncoder(w).Encode(autoauth.Access{Token: "mcp_automatic_secret", OwnerID: user.ID, ConnectionID: user.ID, ExpiresAt: time.Now().Add(24 * time.Hour)})
	}))
	t.Cleanup(srv.Close)
	b.cfg.AutoAuthEnabled = true
	b.cfg.AutoAuthSecret = strings.Repeat("s", 32)
	b.cfg.BuilderURL = srv.URL
	b.cfg.AutoAuthID = "agent"
	run.fn = func(_ context.Context, req agent.Request) (agent.Result, error) {
		return agent.Result{Reply: "готово"}, nil
	}
	return b, api, &calls, &revoked, &down
}

// TestAutoConcurrentRefresh обновляет один PAT для параллельных сообщений одного отправителя.
func TestAutoConcurrentRefresh(t *testing.T) {
	b, api, calls, _, _ := autoFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); b.process(textUpdate(42, "/start")) }()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("обменов %d", calls.Load())
	}
	if auto, _ := b.store.IsAutoAccess(context.Background(), 42); !auto {
		t.Fatal("автоматический доступ не сохранён")
	}
	for _, line := range api.snapshot() {
		if strings.Contains(line.text, "mcp_automatic_secret") || strings.Contains(line.text, b.cfg.AutoAuthSecret) {
			t.Fatal("секрет в Telegram")
		}
	}
	_ = b.store.ExpireAutoAccess(context.Background(), 42)
	b.process(textUpdate(42, "/start"))
	if calls.Load() != 2 {
		t.Fatal("PAT не обновлён")
	}
}

// TestAutoLogoutRecovery отключает доступ до явного восстановления в конструкторе.
func TestAutoLogoutRecovery(t *testing.T) {
	b, _, _, revoked, down := autoFixture(t)
	ctx := context.Background()
	b.process(textUpdate(42, "/start"))
	down.Store(true)
	if err := b.disconnectAuto(ctx, 42); err == nil {
		t.Fatal("ожидалась сетевая ошибка")
	}
	a, _ := b.store.AutoAccess(ctx, 42)
	if !a.PendingDisconnect {
		t.Fatal("отключение не сохранено")
	}
	token, _, _ := b.store.Access(ctx, 42)
	if token == "" {
		t.Fatal("сбой сети удалил доступ")
	}
	sess := &session{state: router.StateMain}
	if b.prepareAutoAccess(textUpdate(42, "задача"), 42, 42, sess) {
		t.Fatal("ожидающее отключение пропустило задачу")
	}
	down.Store(false)
	if b.prepareAutoAccess(textUpdate(42, "задача"), 42, 42, sess) {
		t.Fatal("logout восстановил подключение")
	}
	if !revoked.Load() {
		t.Fatal("сервер не отключён")
	}
	revoked.Store(false)
	if !b.prepareAutoAccess(textUpdate(42, "задача"), 42, 42, sess) {
		t.Fatal("явное восстановление не работает")
	}
}

// TestAutoGroupsAndManualPriority блокирует автоматический PAT в группах, сохраняя ручной.
func TestAutoGroupsAndManualPriority(t *testing.T) {
	b, _, calls, _, _ := autoFixture(t)
	b.process(textUpdate(42, "/start"))
	sess := &session{state: router.StateMain}
	upd := textUpdate(42, "задача от пользователя 99")
	upd.Message.Chat.ID = -100
	if b.prepareAutoAccess(upd, 42, -100, sess) {
		t.Fatal("автоматический доступ в группе")
	}
	cb := callbackUpdate(42, "p:list:0")
	if b.prepareAutoAccess(cb, 42, -100, sess) {
		t.Fatal("автоматическая кнопка в группе")
	}
	if err := b.store.SaveAccess(context.Background(), 42, "mcp_manual", "https://manual.example/mcp"); err != nil {
		t.Fatal(err)
	}
	if !b.prepareAutoAccess(upd, 42, -100, sess) {
		t.Fatal("ручной доступ заблокирован")
	}
	b.process(textUpdate(42, "/start"))
	if calls.Load() != 1 {
		t.Fatal("ручной доступ заменён")
	}
}

// TestAutoNetworkPreservesAccess не удаляет PAT при временной ошибке обновления.
func TestAutoNetworkPreservesAccess(t *testing.T) {
	b, _, _, _, down := autoFixture(t)
	b.process(textUpdate(42, "/start"))
	ctx := context.Background()
	old, _, _ := b.store.Access(ctx, 42)
	_ = b.store.ExpireAutoAccess(ctx, 42)
	down.Store(true)
	if b.prepareAutoAccess(textUpdate(42, "задача"), 42, 42, &session{}) {
		t.Fatal("задача запущена без обновления")
	}
	current, _, _ := b.store.Access(ctx, 42)
	if current != old {
		t.Fatal("сбой удалил PAT")
	}
}
