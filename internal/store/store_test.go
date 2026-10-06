package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestTokenHistoryAndMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.db")
	s, err := Open("sqlite://"+path, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("права %o", info.Mode().Perm())
	}

	ctx := context.Background()
	if err := s.SaveToken(ctx, 7, "mcp_supersecret"); err != nil {
		t.Fatal(err)
	}
	if got := MaskToken("mcp_supersecret"); got == "mcp_supersecret" || got == "не задан" {
		t.Fatal(got)
	}
	tok, err := s.Token(ctx, 7)
	if err != nil || tok != "mcp_supersecret" {
		t.Fatalf("token %q %v", tok, err)
	}
	if err := s.SetActiveProject(ctx, 7, 15); err != nil {
		t.Fatal(err)
	}
	if id, err := s.ActiveProject(ctx, 7); err != nil || id != 15 {
		t.Fatalf("project %d %v", id, err)
	}
	for i := 0; i < 5; i++ {
		if err := s.Append(ctx, 7, []Message{{Role: "user", Content: "m"}}); err != nil {
			t.Fatal(err)
		}
	}
	hist, err := s.History(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 3 {
		t.Fatalf("history %d", len(hist))
	}
	if err := s.ResetHistory(ctx, 7); err != nil {
		t.Fatal(err)
	}
	hist, err = s.History(ctx, 7)
	if err != nil || len(hist) != 0 {
		t.Fatalf("после сброса %+v %v", hist, err)
	}
	tok, err = s.Token(ctx, 7)
	if err != nil || tok != "mcp_supersecret" {
		t.Fatal("сброс истории не должен трогать токен")
	}
	if err := s.SaveAccess(ctx, 7, "mcp_exampletoken", "https://example.com/mcp"); err != nil {
		t.Fatal(err)
	}
	tok, endpoint, err := s.Access(ctx, 7)
	if err != nil || tok != "mcp_exampletoken" || endpoint != "https://example.com/mcp" {
		t.Fatalf("access %q %q %v", tok, endpoint, err)
	}
	if err := s.Logout(ctx, 7); err != nil {
		t.Fatal(err)
	}
	tok, err = s.Token(ctx, 7)
	if err != nil || tok != "" {
		t.Fatalf("logout token %q", tok)
	}
	if id, _ := s.ActiveProject(ctx, 7); id != 0 {
		t.Fatal("logout должен сбросить проект")
	}
	if _, endpoint, _ := s.Access(ctx, 7); endpoint != "" {
		t.Fatal("logout должен сбросить адрес")
	}
	if MaskToken("") != "не задан" {
		t.Fatal("пустая маска")
	}
}

func TestManyUsersKeepTheirRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.db")
	s, err := Open("sqlite://"+path, 40)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var mode string
	if err := s.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal %s", mode)
	}
	for _, suf := range []string{"", "-wal", "-shm"} {
		info, err := os.Stat(path + suf)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s права %o", suf, info.Mode().Perm())
		}
	}

	const users = 40
	const ops = 15
	ctx := context.Background()
	var wg sync.WaitGroup
	errCh := make(chan error, users)
	for u := 1; u <= users; u++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			token := fmt.Sprintf("mcp_user_%d_secret", id)
			if err := s.SaveToken(ctx, id, token); err != nil {
				errCh <- err
				return
			}
			if err := s.SetActiveProject(ctx, id, id*10); err != nil {
				errCh <- err
				return
			}
			for i := 0; i < ops; i++ {
				msg := Message{Role: "user", Content: fmt.Sprintf("u%d-%d", id, i)}
				if err := s.Append(ctx, id, []Message{msg}); err != nil {
					errCh <- err
					return
				}
				if _, err := s.History(ctx, id); err != nil {
					errCh <- err
					return
				}
			}
		}(int64(u))
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}

	for u := 1; u <= users; u++ {
		id := int64(u)
		tok, err := s.Token(ctx, id)
		if err != nil || tok != fmt.Sprintf("mcp_user_%d_secret", id) {
			t.Fatalf("user %d token %q %v", id, tok, err)
		}
		proj, err := s.ActiveProject(ctx, id)
		if err != nil || proj != id*10 {
			t.Fatalf("user %d project %d %v", id, proj, err)
		}
		hist, err := s.History(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(hist) != ops {
			t.Fatalf("user %d history %d", id, len(hist))
		}
		for _, m := range hist {
			if len(m.Content) < 2 || m.Content[0] != 'u' || m.Content[1:] == "" {
				t.Fatalf("user %d чужая строка %q", id, m.Content)
			}
			prefix := fmt.Sprintf("u%d-", id)
			if len(m.Content) < len(prefix) || m.Content[:len(prefix)] != prefix {
				t.Fatalf("user %d чужая строка %q", id, m.Content)
			}
		}
	}
}

func TestSameUserParallelAppendStaysWithinLimit(t *testing.T) {
	dir := t.TempDir()
	s, err := Open("sqlite://"+filepath.Join(dir, "agent.db"), 8)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	const n = 30
	ctx := context.Background()
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := s.Append(ctx, 1, []Message{{Role: "user", Content: fmt.Sprintf("m%d", i)}})
			if err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	hist, err := s.History(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) == 0 || len(hist) > 8 {
		t.Fatalf("history %d", len(hist))
	}
}
