// @fileoverview Проверка сокрытия автоматических секретов на границах вывода.
package text

import (
	"strings"
	"testing"
)

// TestRedactAgentSecrets проверяет PAT и точное значение ключа без удаления данных бота.
func TestRedactAgentSecrets(t *testing.T) {
	key := strings.Repeat("secret", 8)
	t.Setenv("AGENT_AUTO_AUTH_SECRET", key)
	output := RedactAgentSecrets("mcp_abcdef " + key + " ordinary")
	if strings.Contains(output, "mcp_abcdef") || strings.Contains(output, key) || !strings.Contains(output, "ordinary") {
		t.Fatal("секрет не скрыт")
	}
}
