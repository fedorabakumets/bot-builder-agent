/* @fileoverview Совместимая передача confirm для разрешённых опасных действий внутри call_action. */
package agent

import "encoding/json"

// builderConfirmedArguments сохраняет прежнюю политику confirm также для вложенного действия.
func builderConfirmedArguments(name, args string) string {
	if IsDangerous(name) {
		return withConfirm(args)
	}
	if name != "call_action" {
		return args
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal([]byte(args), &envelope) != nil || envelope == nil {
		return args
	}
	var action string
	if json.Unmarshal(envelope["action_id"], &action) != nil || !IsDangerous(action) {
		return args
	}
	envelope["args"] = json.RawMessage(withConfirm(string(envelope["args"])))
	out, err := json.Marshal(envelope)
	if err != nil {
		return args
	}
	return string(out)
}
