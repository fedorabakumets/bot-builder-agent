package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		in, out string
		ok      bool
	}{
		{"https://example.com", "https://example.com/mcp", true},
		{"https://example.com/", "https://example.com/mcp", true},
		{"https://example.com/mcp", "https://example.com/mcp", true},
		{"https://example.com/mcp/", "https://example.com/mcp", true},
		{"http://localhost:3000/app", "http://localhost:3000/app/mcp", true},
		{"example.com", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, err := NormalizeURL(tc.in)
		if tc.ok && (err != nil || got != tc.out) {
			t.Fatalf("%q -> %q err=%v, хотели %q", tc.in, got, err, tc.out)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%q должен быть ошибкой", tc.in)
		}
	}
	if Host("https://example.com/mcp") != "example.com" {
		t.Fatal("host")
	}
}

func TestClientJSONAndSSE(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	inits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, string(body))
		mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			t.Errorf("нет Accept SSE")
		}
		if strings.Contains(string(body), `"method":"initialize"`) {
			inits++
			if r.Header.Get("MCP-Protocol-Version") != "" {
				t.Errorf("initialize не должен слать MCP-Protocol-Version")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26"}}`))
			return
		}
		if strings.Contains(string(body), "notifications/initialized") {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if r.Header.Get("MCP-Protocol-Version") != "2025-03-26" {
			t.Errorf("protocol header: %q", r.Header.Get("MCP-Protocol-Version"))
		}
		if strings.Contains(string(body), "tools/list") {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{\"tools\":[{\"name\":\"db_list_projects\",\"description\":\"список\",\"inputSchema\":{\"type\":\"object\"}}]}}\n\n"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"a"},{"type":"text","text":"b"}],"isError":false}}`))
	}))
	defer srv.Close()

	c, err := New(srv.URL, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := c.ListTools(context.Background(), "", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "db_list_projects" {
		t.Fatalf("tools: %+v", tools)
	}
	tools2, err := c.ListTools(context.Background(), "", "secret")
	if err != nil || len(tools2) != 1 {
		t.Fatal(err)
	}
	if inits != 1 {
		t.Fatalf("initialize вызвался %d раз", inits)
	}
	text, isErr, err := c.CallTool(context.Background(), "", "secret", "db_list_projects", []byte(`{"archived":false}`))
	if err != nil || isErr || text != "a\nb" {
		t.Fatalf("call %q err=%v isErr=%v", text, err, isErr)
	}
}

func TestToolsCursorAndUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("Authorization") != "Bearer ok" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(string(body), "initialize"):
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26"}}`))
		case strings.Contains(string(body), "notifications/initialized"):
			w.WriteHeader(http.StatusAccepted)
		case strings.Contains(string(body), `"cursor":"p2"`):
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"b","inputSchema":{"type":"object"}}]}}`))
		default:
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"a","inputSchema":{"type":"object"}}],"nextCursor":"p2"}}`))
		}
	}))
	defer srv.Close()
	c, err := New(srv.URL, 0)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := c.ListTools(context.Background(), "", "ok")
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 || tools[0].Name != "a" || tools[1].Name != "b" {
		t.Fatalf("%+v", tools)
	}
	_, err = c.ListTools(context.Background(), "", "bad")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("401: %v", err)
	}
}

func TestManyUsersShareOneToolList(t *testing.T) {
	var lists atomic.Int32
	var inits atomic.Int32
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(string(body), `"method":"initialize"`):
			inits.Add(1)
			time.Sleep(20 * time.Millisecond)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26"}}`))
		case strings.Contains(string(body), "notifications/initialized"):
			w.WriteHeader(http.StatusAccepted)
		case strings.Contains(string(body), "tools/list"):
			lists.Add(1)
			time.Sleep(30 * time.Millisecond)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"echo","inputSchema":{"type":"object"}}]}}`))
		default:
			calls.Add(1)
			var req struct {
				Params struct {
					Arguments map[string]any `json:"arguments"`
				} `json:"params"`
			}
			_ = json.Unmarshal(body, &req)
			n, _ := req.Params.Arguments["n"].(float64)
			_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"n-%d"}]}}`, int(n))
		}
	}))
	defer srv.Close()

	c, err := New(srv.URL, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	const n = 30
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx := context.Background()
			token := fmt.Sprintf("tok-%d", i)
			tools, err := c.ListTools(ctx, "", token)
			if err != nil || len(tools) != 1 || tools[0].Name != "echo" {
				errCh <- fmt.Errorf("list %d: %+v %v", i, tools, err)
				return
			}
			got, isErr, err := c.CallTool(ctx, "", token, "echo", []byte(fmt.Sprintf(`{"n":%d}`, i)))
			if err != nil || isErr || got != fmt.Sprintf("n-%d", i) {
				errCh <- fmt.Errorf("call %d: %q %v", i, got, err)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
	if lists.Load() != 1 {
		t.Fatalf("tools/list ушёл %d раз, нужен один общий запрос", lists.Load())
	}
	if inits.Load() != 1 {
		t.Fatalf("initialize ушёл %d раз", inits.Load())
	}
	if calls.Load() != n {
		t.Fatalf("tools/call %d, ждали %d", calls.Load(), n)
	}
}

func TestSnippetURLIsCalled(t *testing.T) {
	defaultHits := 0
	customHits := 0
	def := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defaultHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer def.Close()
	custom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		customHits++
		if r.Header.Get("Authorization") != "Bearer mcp_exampletoken" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(string(body), `"method":"initialize"`) {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26"}}`))
			return
		}
		if strings.Contains(string(body), "notifications/initialized") {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"ok"}]}}`))
	}))
	defer custom.Close()

	c, err := New(def.URL, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"mcpServers":{"botcraft-builder":{"url":"` + custom.URL + `","headers":{"Authorization":"Bearer mcp_exampletoken"}}}}`
	token, endpoint, ok := ParseAccess(raw)
	if !ok || token != "mcp_exampletoken" || endpoint != custom.URL+"/mcp" {
		t.Fatalf("parse %q %q %v", token, endpoint, ok)
	}
	got, _, err := c.CallTool(context.Background(), endpoint, token, "echo", []byte(`{}`))
	if err != nil || got != "ok" {
		t.Fatalf("call %q %v", got, err)
	}
	if customHits == 0 || defaultHits != 0 {
		t.Fatalf("custom %d default %d", customHits, defaultHits)
	}
}

func TestBadRPC(t *testing.T) {
	if _, err := parseRPC("application/json", []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-1,"message":"boom"}}`)); err == nil {
		t.Fatal("ждали ошибку")
	}
	if _, err := parseRPC("application/json", []byte(`{`)); err == nil {
		t.Fatal("ждали битый json")
	}
	if _, err := parseRPC("text/event-stream", []byte("event: message\n\n")); err == nil {
		t.Fatal("ждали пустой sse")
	}
}
