/* @fileoverview Понятные статусы MCP с настоящим именем действия и без содержимого запросов. */
package telegram

import (
	"strings"

	"bot-builder-agent/internal/text"
)

// toolStatus отделяет читаемое действие от его технического имени, сохраняя серверный префикс.
func toolStatus(name string) string {
	action := name
	if index := strings.LastIndex(action, "__"); index >= 0 {
		action = action[index+2:]
	}
	labels := map[string]string{
		"list_sections":       "Смотрю доступные разделы",
		"search_actions":      "Ищу подходящее действие",
		"describe_actions":    "Читаю параметры действий",
		"call_action":         "Выполняю действие",
		"db_list_projects":    "Читаю список проектов",
		"get_project_db":      "Читаю сценарий проекта",
		"get_node_schema":     "Читаю схему ноды",
		"get_node_example":    "Читаю пример ноды",
		"db_get_node":         "Читаю ноду",
		"db_list_nodes":       "Читаю список нод",
		"db_find_nodes":       "Ищу ноды",
		"db_add_node":         "Добавляю ноду",
		"db_update_node":      "Изменяю ноду",
		"db_remove_node":      "Удаляю ноду",
		"db_connect_nodes":    "Соединяю ноды",
		"db_apply_ops":        "Применяю изменения сценария",
		"update_project_db":   "Сохраняю сценарий проекта",
		"db_list_tables":      "Читаю список таблиц",
		"db_list_table_rows":  "Читаю строки таблицы",
		"db_add_table_rows":   "Добавляю строки таблицы",
		"db_edit_table_cells": "Изменяю ячейки таблицы",
		"db_apply_table_ops":  "Применяю изменения таблицы",
		"db_get_analytics_v2": "Читаю аналитику",
		"db_bot_status":       "Проверяю состояние бота",
		"db_start_bot":        "Запускаю бота",
		"db_stop_bot":         "Останавливаю бота",
		"db_restart_bot":      "Перезапускаю бота",
		"db_bot_logs":         "Читаю журнал бота",
	}
	if label := labels[action]; label != "" {
		return label + " · " + text.Redact(name)
	}
	return "Вызываю " + text.Redact(name)
}
