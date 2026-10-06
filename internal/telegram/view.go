package telegram

import (
	"encoding/json"
	"fmt"
	"strings"

	"bot-builder-agent/internal/telegram/ui"
)

type projectEnvelope struct {
	Error    string `json:"error"`
	Projects []struct {
		ID              int64  `json:"id"`
		Name            string `json:"name"`
		IsArchivedForMe bool   `json:"isArchivedForMe"`
	} `json:"projects"`
	ProjectID int64  `json:"projectId"`
	Name      string `json:"name"`
	OK        bool   `json:"ok"`
}

type tokenEnvelope struct {
	Error  string `json:"error"`
	Tokens []struct {
		ID                    int64  `json:"id"`
		Name                  string `json:"name"`
		BotUsername           string `json:"botUsername"`
		MessagesRetentionDays int    `json:"messagesRetentionDays"`
	} `json:"tokens"`
}

// ParseProjects разбирает ответ db_list_projects.
func ParseProjects(raw string) ([]ui.Project, error) {
	var env projectEnvelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &env); err != nil {
		return nil, fmt.Errorf("не разобрать список проектов")
	}
	if env.Error != "" {
		return nil, fmt.Errorf("%s", env.Error)
	}
	out := make([]ui.Project, 0, len(env.Projects))
	for _, p := range env.Projects {
		out = append(out, ui.Project{ID: p.ID, Name: p.Name, Archived: p.IsArchivedForMe})
	}
	return out, nil
}

// ParseCreatedProject разбирает ответ db_create_project.
func ParseCreatedProject(raw string) (int64, string, error) {
	var env projectEnvelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &env); err != nil {
		return 0, "", fmt.Errorf("не разобрать ответ создания проекта")
	}
	if env.Error != "" {
		return 0, "", fmt.Errorf("%s", env.Error)
	}
	if env.ProjectID <= 0 {
		return 0, "", fmt.Errorf("в ответе нет projectId")
	}
	return env.ProjectID, env.Name, nil
}

// ParseTokens разбирает ответ db_list_bot_tokens.
func ParseTokens(raw string) ([]ui.BotToken, error) {
	var env tokenEnvelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &env); err != nil {
		return nil, fmt.Errorf("не разобрать список ботов")
	}
	if env.Error != "" {
		return nil, fmt.Errorf("%s", env.Error)
	}
	out := make([]ui.BotToken, 0, len(env.Tokens))
	for _, t := range env.Tokens {
		out = append(out, ui.BotToken{
			ID:        t.ID,
			Name:      t.Name,
			Username:  t.BotUsername,
			Retention: t.MessagesRetentionDays,
		})
	}
	return out, nil
}
