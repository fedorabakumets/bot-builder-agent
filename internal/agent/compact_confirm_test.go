/* @fileoverview Проверки вложенного confirm: исходные аргументы сохраняются, обычные действия не меняются. */
package agent

import (
	"encoding/json"
	"testing"
)

// TestCompactConfirm добавляет прежний confirm только в args известного опасного действия.
func TestCompactConfirm(t *testing.T) {
	raw := `{"action_id":"db_delete_project","args":{"project_id":7}}`
	var envelope struct {
		Action string         `json:"action_id"`
		Args   map[string]any `json:"args"`
	}
	if err := json.Unmarshal([]byte(builderConfirmedArguments("call_action", raw)), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Action != "db_delete_project" || envelope.Args["confirm"] != true || envelope.Args["project_id"] != float64(7) {
		t.Fatalf("неверный confirm: %+v", envelope)
	}
	for _, value := range []string{`{"action_id":"db_list_projects","args":{}}`, `{"action_id":7}`, `not-json`} {
		if got := builderConfirmedArguments("call_action", value); got != value {
			t.Fatalf("изменены обычные или неверные аргументы: %s", got)
		}
	}
}
