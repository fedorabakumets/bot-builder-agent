// @fileoverview Сброс кэша MCP при обновлении автоматического доступа.
package mcp

// Forget очищает рукопожатие и список инструментов указанного адреса.
func (c *Client) Forget(endpoint string) {
	normalized, err := c.normalize(endpoint)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.states, normalized)
}
