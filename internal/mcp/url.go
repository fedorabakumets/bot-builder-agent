package mcp

import (
	"fmt"
	"net/url"
	"strings"
)

// NormalizeURL приводит адрес конструктора к конечной точке POST /mcp.
func NormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("пустой адрес")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("не разобрать адрес: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("нужна схема http или https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("нет хоста")
	}
	path := strings.TrimRight(u.Path, "/")
	switch {
	case path == "" || path == "/":
		u.Path = "/mcp"
	case strings.HasSuffix(path, "/mcp"):
		u.Path = path
	default:
		u.Path = path + "/mcp"
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

// Host возвращает хост конечной точки без секретов.
func Host(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Host == "" {
		return raw
	}
	return u.Host
}
