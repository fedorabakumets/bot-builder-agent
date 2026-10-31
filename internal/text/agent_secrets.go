// @fileoverview Удаление PAT и межсервисного ключа из логов и контекста модели.
package text

import (
	"os"
	"strings"
)

// RedactAgentSecrets скрывает доступ к конструктору, сохраняя переданные пользователем данные задачи.
func RedactAgentSecrets(s string) string {
	s = mcpTokenRE.ReplaceAllString(s, "[скрытый PAT]")
	if secret := os.Getenv("AGENT_AUTO_AUTH_SECRET"); secret != "" {
		s = strings.ReplaceAll(s, secret, "[скрытый ключ]")
	}
	return s
}
