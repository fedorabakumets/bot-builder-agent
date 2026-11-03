/* @fileoverview Название фактического действия компактного MCP для статуса без раскрытия аргументов. */
package agent

import (
	"encoding/json"
	"regexp"
	"strings"
)

// progressActionID допускает только короткие идентификаторы, исключая разметку и переносы.
var progressActionID = regexp.MustCompile(`^[a-z][a-z0-9_]{0,95}$`)

// toolProgressName разворачивает call_action только у конструктора; исполняемое имя не меняется.
func toolProgressName(name, arguments string, remotes map[string]Remote) string {
	original := name
	if remote, found := remotes[name]; found {
		if !remote.Builder {
			return name
		}
		original = remote.Original
	}
	if original != "call_action" {
		return name
	}
	var envelope struct {
		// ActionID — только имя действия; вложенные аргументы никогда не отображаются.
		ActionID string `json:"action_id"`
	}
	if json.Unmarshal([]byte(arguments), &envelope) != nil || !progressActionID.MatchString(envelope.ActionID) || strings.HasPrefix(envelope.ActionID, "mcp_") {
		return name
	}
	return strings.TrimSuffix(name, original) + envelope.ActionID
}
