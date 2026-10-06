package agent

// Dangerous — инструменты, которые нельзя слать в конструктор без ответа пользователя.
var Dangerous = map[string]struct{}{
	"db_delete_project":     {},
	"db_delete_bot_token":   {},
	"db_delete_version":     {},
	"db_prune_versions":     {},
	"db_stop_bot":           {},
	"db_restart_all_bots":   {},
	"db_start_offline_bots": {},
}

// IsDangerous сообщает, нужен ли отдельный запрос «Да / Нет».
func IsDangerous(name string) bool {
	_, ok := Dangerous[name]
	return ok
}
