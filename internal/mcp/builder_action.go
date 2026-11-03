/* @fileoverview Совместимость кнопок конструктора с полным и компактным MCP без повторного выполнения после ошибки. */
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// CallBuilderAction выбирает опубликованный инструмент до выполнения действия.
// В compact исходное имя передаётся как action_id; результат остаётся прежним.
func (c *Client) CallBuilderAction(ctx context.Context, endpoint, token, name string, args json.RawMessage) (string, bool, error) {
	tools, err := c.ListTools(ctx, endpoint, token)
	if err != nil {
		return "", false, err
	}
	compact := false
	for _, tool := range tools {
		if tool.Name == name {
			return c.CallTool(ctx, endpoint, token, name, args)
		}
		compact = compact || tool.Name == "call_action"
	}
	if !compact {
		return "", false, fmt.Errorf("действие конструктора %s недоступно", name)
	}
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	payload, err := json.Marshal(struct {
		// ActionID — исходное имя разрешённого действия конструктора.
		ActionID string `json:"action_id"`
		// Args — аргументы действия без изменений и дополнительных прав.
		Args json.RawMessage `json:"args"`
	}{ActionID: name, Args: args})
	if err != nil {
		return "", false, err
	}
	return c.CallTool(ctx, endpoint, token, "call_action", payload)
}
