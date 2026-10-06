package agent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"bot-builder-agent/internal/mcp"
	"bot-builder-agent/internal/openrouter"
)

func TestRunnerManyUsersAtOnce(t *testing.T) {
	var lists atomic.Int32
	var calls atomic.Int32
	mcpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(string(body), "initialize"):
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26"}}`)
		case strings.Contains(string(body), "notifications/initialized"):
			w.WriteHeader(http.StatusAccepted)
		case strings.Contains(string(body), "tools/list"):
			lists.Add(1)
			time.Sleep(15 * time.Millisecond)
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"echo","description":"e","inputSchema":{"type":"object"}}]}}`)
		default:
			calls.Add(1)
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"tok:`+token+`"}]}}`)
		}
	}))
	defer mcpSrv.Close()

	orSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(string(body), `"role":"tool"`) {
			writeChoice(w, "готово", "")
			return
		}
		writeChoice(w, "", `{"id":"c1","type":"function","function":{"name":"echo","arguments":"{}"}}`)
	}))
	defer orSrv.Close()

	mcpClient, err := mcp.New(mcpSrv.URL, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	orClient, err := openrouter.NewAt("test-key", orSrv.URL+"/v1", orSrv.Client())
	if err != nil {
		t.Fatal(err)
	}
	runner := &Runner{
		OR:          orClient,
		MCP:         mcpClient,
		Model:       "test",
		MaxRounds:   0,
		MaxTokens:   100,
		Temperature: 0,
	}

	const n = 16
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	var inflight atomic.Int32
	var peak atomic.Int32
	for i := 1; i <= n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			now := inflight.Add(1)
			for {
				old := peak.Load()
				if now <= old || peak.CompareAndSwap(old, now) {
					break
				}
			}
			defer inflight.Add(-1)
			token := fmt.Sprintf("mcp_user_%d", i)
			res, err := runner.Run(context.Background(), Request{
				Token:    token,
				UserText: fmt.Sprintf("задача %d", i),
			})
			if err != nil {
				errCh <- err
				return
			}
			if res.Reply != "готово" {
				errCh <- fmt.Errorf("user %d reply %q", i, res.Reply)
				return
			}
			found := false
			for _, msg := range res.Transcript {
				if msg.Role == "tool" && strings.Contains(msg.Content, "tok:"+token) {
					found = true
				}
				if strings.Contains(msg.Content, "mcp_user_") && !strings.Contains(msg.Content, token) {
					errCh <- fmt.Errorf("user %d увидел чужой токен в %q", i, msg.Content)
					return
				}
			}
			if !found {
				errCh <- fmt.Errorf("user %d не получил свой ответ инструмента", i)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
	if lists.Load() != 1 {
		t.Fatalf("tools/list %d", lists.Load())
	}
	if calls.Load() != int32(n) {
		t.Fatalf("tools/call %d", calls.Load())
	}
	if peak.Load() < int32(n) {
		t.Fatalf("одновременно в агенте было %d из %d", peak.Load(), n)
	}
}
