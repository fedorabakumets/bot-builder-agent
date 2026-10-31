// @fileoverview Проверки очистки памяти прежнего владельца и сохранения памяти при обновлении PAT.
package store

import (
	"bot-builder-agent/internal/autoauth"
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestLogoutClearsOnlyOwnHistory проверяет границу пользователя при выходе.
func TestLogoutClearsOnlyOwnHistory(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "agent.db"), 20)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	for _, id := range []int64{42, 43} {
		if err = s.SaveToken(ctx, id, "mcp_manual"); err != nil {
			t.Fatal(err)
		}
		if err = s.Append(ctx, id, []Message{{Role: "tool", Content: "старые проекты"}}); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Logout(ctx, 42); err != nil {
		t.Fatal(err)
	}
	history, err := s.History(ctx, 42)
	if err != nil || len(history) != 0 {
		t.Fatal("память после выхода сохранилась", err)
	}
	others, err := s.History(ctx, 43)
	if err != nil || len(others) != 1 {
		t.Fatal("память другого пользователя затронута", err)
	}
	token, _, err := s.Access(ctx, 42)
	if err != nil || token != "" {
		t.Fatal("доступ после выхода сохранился", err)
	}
	servers, err := s.ListServers(ctx, 42)
	if err != nil || len(servers) != 0 {
		t.Fatal("сервер конструктора сохранился", err)
	}
}

// TestAutoAccessMemoryBoundary очищает старую память при первом входе, но не при ротации PAT.
func TestAutoAccessMemoryBoundary(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "agent.db"), 20)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err = s.Append(ctx, 42, []Message{{Role: "tool", Content: "чужие проекты"}}); err != nil {
		t.Fatal(err)
	}
	a := autoauth.Access{Token: "mcp_first", ConnectionID: 1, OwnerID: 42, ExpiresAt: time.Now().Add(time.Hour)}
	if err = s.SaveAutoAccess(ctx, 42, "https://example.com/mcp", a); err != nil {
		t.Fatal(err)
	}
	history, err := s.History(ctx, 42)
	if err != nil || len(history) != 0 {
		t.Fatal("старая память перенесена", err)
	}
	if err = s.Append(ctx, 42, []Message{{Role: "user", Content: "новая задача"}}); err != nil {
		t.Fatal(err)
	}
	a.Token = "mcp_rotated"
	if err = s.SaveAutoAccess(ctx, 42, "https://example.com/mcp", a); err != nil {
		t.Fatal(err)
	}
	history, err = s.History(ctx, 42)
	if err != nil || len(history) != 1 {
		t.Fatal("ротация потеряла диалог", err)
	}
}
