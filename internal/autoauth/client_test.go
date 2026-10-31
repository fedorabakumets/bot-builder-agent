// @fileoverview Проверки подписи, запрета редиректов и безопасных ошибок обмена.
package autoauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestExchangeSignature проверяет фактическое тело и подтверждение владельца.
func TestExchangeSignature(t *testing.T) {
	secret := strings.Repeat("s", 32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		want := Signature(secret, r.Method, r.URL.Path, r.Header.Get("X-Agent-Timestamp"), r.Header.Get("X-Agent-Nonce"), body)
		if r.Header.Get("X-Agent-Signature") != want {
			t.Error("неверная подпись")
		}
		var user Identity
		_ = json.Unmarshal(body, &user)
		if user.ID != 42 {
			t.Error("неверный отправитель")
		}
		_ = json.NewEncoder(w).Encode(Access{Token: "mcp_test", OwnerID: 42, ConnectionID: 1, ExpiresAt: time.Now().Add(time.Hour)})
	}))
	defer srv.Close()
	c := Client{Endpoint: srv.URL + "/mcp", AgentID: "agent", Secret: secret}
	if _, err := c.Exchange(context.Background(), Identity{ID: 42, FirstName: "Тест"}); err != nil {
		t.Fatal(err)
	}
}

// TestNoRedirectOrRetry исключает передачу подписи другому серверу и повторы запросов.
func TestNoRedirectOrRetry(t *testing.T) {
	calls := 0
	leaked := 0
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked++ }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; http.Redirect(w, r, other.URL, 307) }))
	defer srv.Close()
	c := Client{Endpoint: srv.URL, Secret: strings.Repeat("s", 32)}
	if _, err := c.Exchange(context.Background(), Identity{ID: 1}); err != ErrUnavailable {
		t.Fatal(err)
	}
	if calls != 1 || leaked != 0 {
		t.Fatalf("запросы %d, утечки %d", calls, leaked)
	}
}

// TestSafeFailure не раскрывает текст ошибок сервера или ключ.
func TestSafeFailure(t *testing.T) {
	for _, code := range []string{"CONNECTION_REVOKED", "mcp_secret"} {
		t.Run(code, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(403)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
			}))
			defer srv.Close()
			c := Client{Endpoint: srv.URL, Secret: strings.Repeat("s", 32)}
			_, err := c.Exchange(context.Background(), Identity{ID: 1})
			if code == "CONNECTION_REVOKED" && err != ErrRevoked {
				t.Fatal(err)
			}
			if code != "CONNECTION_REVOKED" && err != ErrUnavailable {
				t.Fatal(err)
			}
		})
	}
}
