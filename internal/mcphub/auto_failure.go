// @fileoverview Автоматический доступ: остановка задачи после неоднозначной ошибки MCP.
package mcphub

import (
	"bot-builder-agent/internal/mcp"
	"bot-builder-agent/internal/store"
	"context"
	"errors"
)

// autoFailure не позволяет модели повторить изменяющий вызов и обновляет отозванный PAT позже.
func (h *Hub) autoFailure(ctx context.Context, srv store.Server, callErr error) error {
	automatic, err := h.Store.IsAutoAccess(ctx, srv.UserID)
	if err != nil {
		return errors.New("не удалось проверить подключение")
	}
	if !srv.Builder || !automatic {
		return nil
	}
	h.dropSession(srv)
	if oauthUnauthorized(callErr) {
		_ = h.Store.ExpireAutoAccess(ctx, srv.UserID)
		return mcp.ErrUnauthorized
	}
	return errors.New("связь с конструктором прервана; проверьте результат перед повтором изменения")
}
