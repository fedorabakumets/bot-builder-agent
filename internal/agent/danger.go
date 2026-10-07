package agent

// Dangerous — инструменты, которым конструктор требует confirm: true.
// Агент подставляет это поле сам и вызывает их сразу, без вопроса пользователю.
var Dangerous = map[string]struct{}{
	"db_delete_project":     {},
	"db_delete_bot_token":   {},
	"db_delete_version":     {},
	"db_prune_versions":     {},
	"db_stop_bot":           {},
	"db_restart_all_bots":   {},
	"db_start_offline_bots": {},
}

// IsDangerous сообщает, нужно ли добавить confirm: true перед вызовом.
func IsDangerous(name string) bool {
	_, ok := Dangerous[name]
	return ok
}
