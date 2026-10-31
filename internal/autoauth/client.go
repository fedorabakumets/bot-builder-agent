// @fileoverview Подписанный обмен Telegram-личности на персональный доступ конструктора.
package autoauth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Identity содержит исключительно сведения из Telegram Update.
type Identity struct {
	// ID отправителя.
	ID int64 `json:"id"`
	// Имя отправителя.
	FirstName string `json:"firstName"`
	// Фамилия, если известна.
	LastName string `json:"lastName,omitempty"`
	// Имя пользователя, если известно.
	Username string `json:"username,omitempty"`
}

// Access — результат серверного обмена; секрет не передаётся модели.
type Access struct {
	// Персональный токен.
	Token string `json:"token"`
	// Срок действия.
	ExpiresAt time.Time `json:"expiresAt"`
	// Идентификатор подключения.
	ConnectionID int64 `json:"connectionId"`
	// Подтверждённый владелец.
	OwnerID int64 `json:"ownerId"`
}

// Client обращается только к адресу, заданному оператором сервиса.
type Client struct {
	// Фиксированный адрес конструктора.
	Endpoint string
	// Идентификатор доверенного агента.
	AgentID string
	// Межсервисный ключ подписи.
	Secret string
}

// ErrRevoked обозначает явное отключение пользователем.
var ErrRevoked = errors.New("подключение отключено; включите его во вкладке «Агент» конструктора")

// ErrUnavailable не раскрывает детали сети, подписи или тело ответа.
var ErrUnavailable = errors.New("автоматическое подключение временно недоступно; повторите позже")

// Signature подписывает те же байты, которые отправляются в HTTP.
func Signature(secret, method, path, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.Join([]string{method, path, timestamp, nonce, hex.EncodeToString(digest[:])}, "\n")))
	return hex.EncodeToString(mac.Sum(nil))
}

// call выполняет один запрос без перенаправлений и автоматических повторов.
func (c Client) call(ctx context.Context, action string, payload any, out any) error {
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		(u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost"))) || len(c.Secret) < 32 {
		return ErrUnavailable
	}
	u.Path = "/api/auth/agent/" + action
	u.RawPath = ""
	body, err := json.Marshal(payload)
	if err != nil {
		return ErrUnavailable
	}
	random := make([]byte, 16)
	if _, err = rand.Read(random); err != nil {
		return ErrUnavailable
	}
	nonce, stamp := hex.EncodeToString(random), strconv.FormatInt(time.Now().Unix(), 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return ErrUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Id", c.AgentID)
	req.Header.Set("X-Agent-Timestamp", stamp)
	req.Header.Set("X-Agent-Nonce", nonce)
	req.Header.Set("X-Agent-Signature", Signature(c.Secret, http.MethodPost, u.Path, stamp, nonce, body))
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var failure struct {
			Code string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&failure)
		if failure.Code == "CONNECTION_REVOKED" {
			return ErrRevoked
		}
		return ErrUnavailable
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(out); err != nil {
		return ErrUnavailable
	}
	return nil
}

// Exchange получает PAT для подтверждённого отправителя.
func (c Client) Exchange(ctx context.Context, user Identity) (Access, error) {
	var access Access
	err := c.call(ctx, "telegram", user, &access)
	if err == nil && (access.OwnerID != user.ID || access.ConnectionID <= 0 || !strings.HasPrefix(access.Token, "mcp_") || !access.ExpiresAt.After(time.Now())) {
		err = ErrUnavailable
	}
	return access, err
}

// Disconnect навсегда отключает автоматический доступ до действия в панели.
func (c Client) Disconnect(ctx context.Context, id int64) error {
	var out struct {
		Success bool `json:"success"`
	}
	err := c.call(ctx, "disconnect", map[string]int64{"id": id}, &out)
	if err == nil && !out.Success {
		return ErrUnavailable
	}
	return err
}
