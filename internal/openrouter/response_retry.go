// @fileoverview Безопасные повторы пустых ответов и диагностика без содержимого диалога.
package openrouter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// ResponseError описывает непригодный ответ модели без секретов запроса.
type ResponseError struct {
	// Code — стабильный код причины.
	Code string
	// Reason — причина завершения генерации.
	Reason string
	// Attempts — число выполненных запросов.
	Attempts int
}

// Error возвращает понятную диагностику, не утверждая успешность изменений.
func (e *ResponseError) Error() string {
	if e.Code == "MODEL_CONTENT_FILTER" {
		return "MODEL_CONTENT_FILTER: провайдер остановил ответ фильтром содержимого. Автоматический повтор не выполнялся."
	}
	if e.Code == "MODEL_TOKEN_LIMIT" {
		return "MODEL_TOKEN_LIMIT: модель исчерпала лимит токенов; неполный ответ не выполнен. Ранее выполненные действия могли сохраниться."
	}
	return fmt.Sprintf("%s: модель не вернула текст или вызовы инструментов после %d запросов (причина: %s). Ранее выполненные действия могли сохраниться; повторно они не запускались.", e.Code, e.Attempts, e.Reason)
}

// Complete повторяет только пустую генерацию, сохраняя исходные результаты инструментов.
func (c *Client) Complete(ctx context.Context, req Request) (Message, error) {
	for attempt := 1; attempt <= 3; attempt++ {
		started := time.Now()
		out, err := c.completeOnce(ctx, req)
		slog.Info("запрос модели завершён", "model", req.Model, "attempt", attempt, "duration_ms", time.Since(started).Milliseconds(), "ok", err == nil)
		var responseErr *ResponseError
		if !errors.As(err, &responseErr) {
			return out, err
		}
		responseErr.Attempts = attempt
		if responseErr.Code != "MODEL_EMPTY_RESPONSE" || attempt == 3 {
			return Message{}, responseErr
		}
		slog.Warn("повтор пустого ответа модели", "model", req.Model, "attempt", attempt, "code", responseErr.Code)
		timer := time.NewTimer(time.Duration(attempt) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return Message{}, ctx.Err()
		case <-timer.C:
		}
	}
	panic("недостижимая ветка повторов")
}

// logCompletion пишет только технические метаданные, без текста и аргументов инструментов.
func logCompletion(model string, resp openai.ChatCompletionResponse) {
	reason := "no_choices"
	if len(resp.Choices) > 0 {
		reason = string(resp.Choices[0].FinishReason)
	}
	slog.Info("ответ модели", "model", model, "response_id", resp.ID, "finish_reason", reason,
		"prompt_tokens", resp.Usage.PromptTokens, "completion_tokens", resp.Usage.CompletionTokens)
}
