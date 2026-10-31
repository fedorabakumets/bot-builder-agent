// @fileoverview Хранение признака автоматического доступа и ожидающего отключения.
package store

import (
	"bot-builder-agent/internal/autoauth"
	"bot-builder-agent/internal/mcp"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"
)

// AutoAccess хранит происхождение текущего токена между перезапусками.
type AutoAccess struct {
	// Хеш токена для отличия последующего ручного подключения.
	TokenHash string
	// Время истечения PAT.
	ExpiresAt time.Time
	// Серверный идентификатор подключения.
	ConnectionID int64
	// Требуется повторить отключение после сетевой ошибки.
	PendingDisconnect bool
}

// TokenHash позволяет сравнить источник, не дублируя секрет.
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// migrateAutoAccess создаёт дополнительную таблицу без изменения существующих подключений.
func (s *Store) migrateAutoAccess() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS auto_access (
 telegram_user_id INTEGER PRIMARY KEY, token_hash TEXT NOT NULL,
 expires_at TEXT NOT NULL, connection_id INTEGER NOT NULL,
 pending_disconnect INTEGER NOT NULL DEFAULT 0)`)
	return err
}

// AutoAccess читает происхождение PAT и состояние отложенного отключения.
func (s *Store) AutoAccess(ctx context.Context, id int64) (AutoAccess, error) {
	var a AutoAccess
	var expires string
	err := s.db.QueryRowContext(ctx, `SELECT token_hash,expires_at,connection_id,pending_disconnect FROM auto_access WHERE telegram_user_id=?`, id).Scan(&a.TokenHash, &expires, &a.ConnectionID, &a.PendingDisconnect)
	if err == sql.ErrNoRows {
		return a, nil
	}
	if err != nil {
		return a, err
	}
	a.ExpiresAt, err = time.Parse(time.RFC3339Nano, expires)
	return a, err
}

// SaveAutoAccess атомарно сохраняет PAT, сервер MCP и происхождение подключения.
func (s *Store) SaveAutoAccess(ctx context.Context, id int64, endpoint string, a autoauth.Access) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO users(telegram_user_id,mcp_token,mcp_url,updated_at) VALUES(?,?,?,?)
 ON CONFLICT(telegram_user_id) DO UPDATE SET mcp_token=excluded.mcp_token,mcp_url=excluded.mcp_url,updated_at=excluded.updated_at`, id, a.Token, endpoint, now)
	if err != nil {
		return err
	}
	raw, err := (mcp.Credentials{Token: a.Token}).JSON()
	if err != nil {
		return err
	}
	item := Server{UserID: id, Name: mcp.BuilderName, URL: endpoint, Transport: "streamable", AuthKind: mcp.AuthBearer, AuthJSON: raw, Enabled: true, Builder: true}
	_, err = tx.ExecContext(ctx, `INSERT INTO mcp_servers(telegram_user_id,name,url,transport,auth_kind,auth_json,enabled,builder,revision,created_at,updated_at)
 VALUES(?,?,?,?,?,?,1,1,?,?,?) ON CONFLICT(telegram_user_id,name) DO UPDATE SET url=excluded.url,
 transport=excluded.transport,auth_kind=excluded.auth_kind,auth_json=excluded.auth_json,enabled=1,builder=1,revision=excluded.revision,updated_at=excluded.updated_at`,
		id, item.Name, endpoint, item.Transport, item.AuthKind, raw, revisionOf(item), now, now)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO auto_access(telegram_user_id,token_hash,expires_at,connection_id,pending_disconnect)
 VALUES(?,?,?,?,0) ON CONFLICT(telegram_user_id) DO UPDATE SET token_hash=excluded.token_hash,
 expires_at=excluded.expires_at,connection_id=excluded.connection_id,pending_disconnect=0`,
		id, TokenHash(a.Token), a.ExpiresAt.Format(time.RFC3339Nano), a.ConnectionID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ExpireAutoAccess заставляет следующий запрос обновить отозванный PAT без повтора операции.
func (s *Store) ExpireAutoAccess(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE auto_access SET expires_at=? WHERE telegram_user_id=?`, time.Unix(0, 0).UTC().Format(time.RFC3339Nano), id)
	return err
}

// MarkAutoDisconnect сохраняет запрет использования до подтверждённого отключения на сервере.
func (s *Store) MarkAutoDisconnect(ctx context.Context, id int64, pending bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE auto_access SET pending_disconnect=? WHERE telegram_user_id=?`, pending, id)
	return err
}

// IsAutoAccess отличает вручную заменённый токен от автоматического.
func (s *Store) IsAutoAccess(ctx context.Context, id int64) (bool, error) {
	a, err := s.AutoAccess(ctx, id)
	if err != nil {
		return false, err
	}
	token, _, err := s.Access(ctx, id)
	if err != nil {
		return false, err
	}
	return a.ConnectionID != 0 && (token == "" || TokenHash(token) == a.TokenHash), nil
}
