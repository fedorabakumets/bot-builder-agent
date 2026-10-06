package telegram

import "testing"

func TestParseProjectsAndTokens(t *testing.T) {
	projects, err := ParseProjects(`{"total":1,"projects":[{"id":4,"name":"Магазин","isArchivedForMe":false}]}`)
	if err != nil || len(projects) != 1 || projects[0].ID != 4 || projects[0].Name != "Магазин" {
		t.Fatalf("%+v %v", projects, err)
	}
	if _, err := ParseProjects(`{"error":"HTTP 401"}`); err == nil {
		t.Fatal("ждали ошибку")
	}
	id, name, err := ParseCreatedProject(`{"ok":true,"projectId":9,"name":"Новый"}`)
	if err != nil || id != 9 || name != "Новый" {
		t.Fatalf("%d %s %v", id, name, err)
	}
	tokens, err := ParseTokens(`{"total":1,"tokens":[{"id":2,"name":"main","botUsername":"shop","messagesRetentionDays":30}]}`)
	if err != nil || len(tokens) != 1 || tokens[0].ID != 2 || tokens[0].Retention != 30 {
		t.Fatalf("%+v %v", tokens, err)
	}
}
