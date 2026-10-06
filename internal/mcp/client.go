package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	protocolVersion = "2025-03-26"
	clientName      = "bot-builder-agent"
	clientVersion   = "0.1.0"
)

// ErrUnauthorized — конструктор отклонил Bearer-токен.
var ErrUnauthorized = errors.New("mcp: токен отклонён")

// Tool — описание инструмента с remote MCP.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Client ходит в stateless POST /mcp.
type Client struct {
	endpoint string
	http     *http.Client
	cacheTTL time.Duration

	mu     sync.Mutex
	states map[string]*endpointState

	nextID atomic.Int64
}

// endpointState держит кэш и рукопожатие отдельно для каждого url из токена.
type endpointState struct {
	initialized bool
	protocol    string
	tools       []Tool
	toolsAt     time.Time
	listFlight  *callFlight
	initFlight  *callFlight
}

// callFlight собирает одновременные одинаковые запросы в один поход на сервер.
type callFlight struct {
	done  chan struct{}
	tools []Tool
	err   error
	token string
}

// New создаёт клиент. cacheTTL задаёт, как долго держать tools/list.
func New(endpoint string, cacheTTL time.Duration) (*Client, error) {
	normalized, err := NormalizeURL(endpoint)
	if err != nil {
		return nil, err
	}
	if cacheTTL < 0 {
		cacheTTL = 0
	}
	return &Client{
		endpoint: normalized,
		cacheTTL: cacheTTL,
		states:   map[string]*endpointState{},
		http: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Endpoint возвращает нормализованный URL.
func (c *Client) Endpoint() string { return c.endpoint }

// ListTools возвращает полный список, обходя nextCursor. Результат кэшируется.
// Одновременные промахи кэша делают один запрос, а не по запросу на чат.
func (c *Client) ListTools(ctx context.Context, endpoint, token string) ([]Tool, error) {
	endpoint, err := c.normalize(endpoint)
	if err != nil {
		return nil, err
	}
	for {
		c.mu.Lock()
		st := c.ensureState(endpoint)
		if out, ok := cachedTools(st, c.cacheTTL); ok {
			c.mu.Unlock()
			return out, nil
		}
		if st.listFlight != nil {
			flight := st.listFlight
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-flight.done:
			}
			if flight.err == nil {
				return append([]Tool(nil), flight.tools...), nil
			}
			if flight.token == token {
				return nil, flight.err
			}
			continue
		}
		flight := &callFlight{done: make(chan struct{}), token: token}
		st.listFlight = flight
		c.mu.Unlock()

		tools, err := c.fetchTools(ctx, endpoint, token)
		c.mu.Lock()
		st = c.ensureState(endpoint)
		if err == nil {
			st.tools = append([]Tool(nil), tools...)
			st.toolsAt = time.Now()
		}
		if st.listFlight == flight {
			st.listFlight = nil
		}
		flight.tools = tools
		flight.err = err
		c.mu.Unlock()
		close(flight.done)
		if err != nil {
			return nil, err
		}
		return append([]Tool(nil), tools...), nil
	}
}

func cachedTools(st *endpointState, ttl time.Duration) ([]Tool, bool) {
	if len(st.tools) > 0 && time.Since(st.toolsAt) < ttl {
		return append([]Tool(nil), st.tools...), true
	}
	return nil, false
}

func (c *Client) fetchTools(ctx context.Context, endpoint, token string) ([]Tool, error) {
	if err := c.ensureInit(ctx, endpoint, token); err != nil {
		return nil, err
	}
	var all []Tool
	cursor := ""
	for page := 0; page < 20; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := c.call(ctx, endpoint, token, "tools/list", params, true)
		if err != nil {
			return nil, err
		}
		var parsed struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("битый tools/list: %w", err)
		}
		all = append(all, parsed.Tools...)
		if parsed.NextCursor == "" || parsed.NextCursor == cursor {
			break
		}
		cursor = parsed.NextCursor
	}
	return all, nil
}

// CallTool выполняет tools/call и склеивает текстовые блоки content.
func (c *Client) CallTool(ctx context.Context, endpoint, token, name string, args json.RawMessage) (string, bool, error) {
	endpoint, err := c.normalize(endpoint)
	if err != nil {
		return "", false, err
	}
	if err := c.ensureInit(ctx, endpoint, token); err != nil {
		return "", false, err
	}
	var arguments any
	trimmed := bytes.TrimSpace(args)
	if len(trimmed) == 0 {
		arguments = map[string]any{}
	} else if err := json.Unmarshal(trimmed, &arguments); err != nil {
		return "", false, fmt.Errorf("аргументы %s не JSON: %w", name, err)
	}
	raw, err := c.call(ctx, endpoint, token, "tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	}, true)
	if err != nil {
		return "", false, err
	}
	return joinContent(raw)
}

func (c *Client) ensureInit(ctx context.Context, endpoint, token string) error {
	for {
		c.mu.Lock()
		st := c.ensureState(endpoint)
		if st.initialized {
			c.mu.Unlock()
			return nil
		}
		if st.initFlight != nil {
			flight := st.initFlight
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-flight.done:
			}
			if flight.err == nil {
				return nil
			}
			if flight.token == token {
				return flight.err
			}
			continue
		}
		flight := &callFlight{done: make(chan struct{}), token: token}
		st.initFlight = flight
		c.mu.Unlock()

		err := c.doInit(ctx, endpoint, token)
		c.mu.Lock()
		st = c.ensureState(endpoint)
		if st.initFlight == flight {
			st.initFlight = nil
		}
		flight.err = err
		c.mu.Unlock()
		close(flight.done)
		return err
	}
}

func (c *Client) doInit(ctx context.Context, endpoint, token string) error {
	raw, err := c.call(ctx, endpoint, token, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    clientName,
			"version": clientVersion,
		},
	}, false)
	if err != nil {
		return err
	}
	var parsed struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(raw, &parsed)
	version := protocolVersion
	if strings.TrimSpace(parsed.ProtocolVersion) != "" {
		version = parsed.ProtocolVersion
	}
	c.mu.Lock()
	st := c.ensureState(endpoint)
	st.initialized = true
	st.protocol = version
	c.mu.Unlock()
	_ = c.notify(ctx, endpoint, token, "notifications/initialized")
	return nil
}

func (c *Client) notify(ctx context.Context, endpoint, token, method string) error {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	})
	if err != nil {
		return err
	}
	status, _, err := c.post(ctx, endpoint, token, body, true)
	if err != nil {
		return err
	}
	if status == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	return nil
}

func (c *Client) call(ctx context.Context, endpoint, token, method string, params any, withProtocol bool) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	status, respBody, contentType, err := c.postFull(ctx, endpoint, token, body, withProtocol)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("mcp http %d", status)
	}
	return parseRPC(contentType, respBody)
}

func (c *Client) post(ctx context.Context, endpoint, token string, body []byte, withProtocol bool) (int, []byte, error) {
	status, resp, _, err := c.postFull(ctx, endpoint, token, body, withProtocol)
	return status, resp, err
}

func (c *Client) normalize(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return c.endpoint, nil
	}
	return NormalizeURL(endpoint)
}

func (c *Client) ensureState(endpoint string) *endpointState {
	st := c.states[endpoint]
	if st == nil {
		st = &endpointState{protocol: protocolVersion}
		c.states[endpoint] = st
	}
	return st
}

func (c *Client) postFull(ctx context.Context, endpoint, token string, body []byte, withProtocol bool) (int, []byte, string, error) {
	endpoint, err := c.normalize(endpoint)
	if err != nil {
		return 0, nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	if withProtocol {
		c.mu.Lock()
		version := protocolVersion
		if st := c.states[endpoint]; st != nil && st.protocol != "" {
			version = st.protocol
		}
		c.mu.Unlock()
		req.Header.Set("MCP-Protocol-Version", version)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return resp.StatusCode, nil, resp.Header.Get("Content-Type"), err
	}
	return resp.StatusCode, respBody, resp.Header.Get("Content-Type"), nil
}
