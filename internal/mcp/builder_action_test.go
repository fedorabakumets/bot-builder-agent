/* @fileoverview Проверки полного и компактного MCP до единственного выполнения действия. */
package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestBuilderActionProfiles проверяет прежние кнопки без публикации полного каталога.
func TestBuilderActionProfiles(t *testing.T) {
	for _, compact := range []bool{false, true} {
		t.Run(map[bool]string{false: "full", true: "compact"}[compact], func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("потерян контекст токена")
				}
				var rpc struct {
					ID     any             `json:"id"`
					Method string          `json:"method"`
					Params json.RawMessage `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&rpc); err != nil {
					t.Error(err)
					return
				}
				var result any
				switch rpc.Method {
				case "initialize":
					result = map[string]any{"protocolVersion": "2025-03-26"}
				case "notifications/initialized":
					w.WriteHeader(202)
					return
				case "tools/list":
					name := "db_list_projects"
					if compact {
						name = "call_action"
					}
					result = map[string]any{"tools": []any{map[string]any{"name": name, "inputSchema": map[string]any{"type": "object"}}}}
				case "tools/call":
					calls++
					var params struct {
						Name string                     `json:"name"`
						Args map[string]json.RawMessage `json:"arguments"`
					}
					if err := json.Unmarshal(rpc.Params, &params); err != nil {
						t.Error(err)
					}
					if compact {
						if params.Name != "call_action" || string(params.Args["action_id"]) != `"db_list_projects"` {
							t.Error("неверное делегирование")
						}
						var args map[string]any
						_ = json.Unmarshal(params.Args["args"], &args)
						if args["archived"] != true {
							t.Error("потерян аргумент действия")
						}
					} else if params.Name != "db_list_projects" || string(params.Args["archived"]) != "true" {
						t.Error("изменён прямой вызов")
					}
					result = map[string]any{"content": []any{map[string]any{"type": "text", "text": `[{"id":1}]`}}}
				default:
					t.Errorf("неизвестный метод %s", rpc.Method)
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": rpc.ID, "result": result})
			}))
			defer srv.Close()
			client, err := New(srv.URL, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			out, failed, err := client.CallBuilderAction(context.Background(), srv.URL, "fixture", "db_list_projects", json.RawMessage(`{"archived":true}`))
			if err != nil || failed || out != `[{"id":1}]` || calls != 1 {
				t.Fatalf("out=%s failed=%v err=%v calls=%d", out, failed, err, calls)
			}
		})
	}
}
