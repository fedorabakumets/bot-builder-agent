// @fileoverview Проверка сохранения автоматического доступа и приоритета ручного подключения.
package store

import (
	"bot-builder-agent/internal/autoauth"
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestAutoAccessRestart проверяет атомарное сохранение, перезапуск, изоляцию и ручной приоритет.
func TestAutoAccessRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.db")
	ctx := context.Background()
	s, err := Open(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	a := autoauth.Access{Token: "mcp_auto", ConnectionID: 4, OwnerID: 42, ExpiresAt: time.Now().Add(time.Hour)}
	if err = s.SaveAutoAccess(ctx, 42, "https://builder.example/mcp", a); err != nil {
		t.Fatal(err)
	}
	if err = s.MarkAutoDisconnect(ctx, 42, true); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	s, err = Open(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	saved, err := s.AutoAccess(ctx, 42)
	if err != nil || !saved.PendingDisconnect || saved.ConnectionID != 4 {
		t.Fatalf("метаданные %+v %v", saved, err)
	}
	token, endpoint, err := s.Access(ctx, 42)
	if err != nil || token != a.Token || endpoint != "https://builder.example/mcp" {
		t.Fatal("доступ потерян")
	}
	servers, err := s.ListServers(ctx, 42)
	if err != nil || len(servers) != 1 || !servers[0].Builder {
		t.Fatal("сервер не сохранён")
	}
	if token, _, _ := s.Access(ctx, 43); token != "" {
		t.Fatal("доступ другого пользователя")
	}
	if err = s.SaveAccess(ctx, 42, "mcp_manual", "https://manual.example/mcp"); err != nil {
		t.Fatal(err)
	}
	if auto, _ := s.IsAutoAccess(ctx, 42); auto {
		t.Fatal("ручной доступ заменён")
	}
}
