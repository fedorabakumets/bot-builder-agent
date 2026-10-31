// @fileoverview Атомарное удаление доступа и памяти прежнего подключения.
package store

import (
	"context"
	"database/sql"
	"time"
)

// Logout удаляет доступ к конструктору и историю, сохраняя остальные серверы пользователя.
func (s *Store) Logout(ctx context.Context, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err = tx.ExecContext(ctx, `UPDATE users SET mcp_token='',mcp_url='',active_project_id=0,updated_at=? WHERE telegram_user_id=?`, now, userID); err != nil {
		return err
	}
	for _, query := range []string{
		`DELETE FROM mcp_servers WHERE telegram_user_id=? AND builder=1`,
		`DELETE FROM messages WHERE telegram_user_id=?`,
	} {
		if _, err = tx.ExecContext(ctx, query, userID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// currentTokenHash возвращает хеш текущего доступа, не раскрывая секрет за пределами хранилища.
func currentTokenHash(ctx context.Context, tx *sql.Tx, id int64) string {
	var token string
	if err := tx.QueryRowContext(ctx, `SELECT mcp_token FROM users WHERE telegram_user_id=?`, id).Scan(&token); err != nil {
		return ""
	}
	return TokenHash(token)
}
