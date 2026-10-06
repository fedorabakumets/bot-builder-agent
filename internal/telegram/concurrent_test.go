package telegram

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mymmrac/telego"

	"bot-builder-agent/internal/agent"
	"bot-builder-agent/internal/config"
	"bot-builder-agent/internal/logger"
	"bot-builder-agent/internal/mcp"
	"bot-builder-agent/internal/openrouter"
	"bot-builder-agent/internal/store"
)

func TestManyChatsRunTogether(t *testing.T) {
	b, api, run := newTestBot(t)
	const n = 24
	entered := make(chan struct{}, n)
	release := make(chan struct{})
	var inflight atomic.Int32
	var peak atomic.Int32
	run.fn = func(ctx context.Context, req agent.Request) (agent.Result, error) {
		now := inflight.Add(1)
		for {
			old := peak.Load()
			if now <= old || peak.CompareAndSwap(old, now) {
				break
			}
		}
		defer inflight.Add(-1)
		entered <- struct{}{}
		select {
		case <-ctx.Done():
			return agent.Result{}, ctx.Err()
		case <-release:
		}
		return agent.Result{
			Reply:      "готово " + req.UserText,
			Transcript: []openrouter.Message{{Role: "user", Content: req.UserText}},
		}, nil
	}

	ctx := context.Background()
	for i := 1; i <= n; i++ {
		if err := b.store.SaveToken(ctx, int64(i), fmt.Sprintf("mcp_user_%d_xx", i)); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	for i := 1; i <= n; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			b.process(textUpdate(id, fmt.Sprintf("задача %d", id)))
		}(int64(i))
	}
	for i := 0; i < n; i++ {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatalf("в агента вошло %d из %d", i, n)
		}
	}
	if peak.Load() != int32(n) {
		t.Fatalf("одновременно работало %d чатов из %d", peak.Load(), n)
	}

	var busy sync.WaitGroup
	for i := 1; i <= n; i++ {
		busy.Add(1)
		go func(id int64) {
			defer busy.Done()
			b.process(textUpdate(id, "второе сообщение"))
		}(int64(i))
	}
	busy.Wait()
	close(release)
	wg.Wait()

	texts := api.snapshot()
	for i := 1; i <= n; i++ {
		own := fmt.Sprintf("готово задача %d", i)
		other := fmt.Sprintf("готово задача %d", i%n+1)
		if !hasText(texts, int64(i), own) {
			t.Errorf("чат %d не получил свой ответ", i)
		}
		if hasText(texts, int64(i), other) {
			t.Errorf("чат %d получил чужой ответ %s", i, other)
		}
		if !hasText(texts, int64(i), "Уже работаю над предыдущим запросом.") {
			t.Errorf("чат %d принял второе сообщение, пока шёл ответ", i)
		}
	}
}

func TestStopOneChatDoesNotStopAnother(t *testing.T) {
	b, api, run := newTestBot(t)
	entered := make(chan int64, 2)
	run.fn = func(ctx context.Context, req agent.Request) (agent.Result, error) {
		var id int64
		fmt.Sscanf(req.UserText, "задача %d", &id)
		entered <- id
		<-ctx.Done()
		return agent.Result{}, ctx.Err()
	}
	ctx := context.Background()
	for _, id := range []int64{1, 2} {
		if err := b.store.SaveToken(ctx, id, fmt.Sprintf("mcp_user_%d_xx", id)); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		b.process(textUpdate(1, "задача 1"))
	}()
	go func() {
		defer wg.Done()
		b.process(textUpdate(2, "задача 2"))
	}()
	got := map[int64]bool{}
	timeout := time.After(3 * time.Second)
	for len(got) < 2 {
		select {
		case id := <-entered:
			got[id] = true
		case <-timeout:
			t.Fatal("оба чата должны были зайти в агента")
		}
	}
	b.process(callbackUpdate(1, "run:stop"))
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("остановка первого чата завершила и второй")
	case <-time.After(150 * time.Millisecond):
	}
	b.process(callbackUpdate(2, "run:stop"))
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("второй чат не остановился")
	}
	texts := api.snapshot()
	if !hasText(texts, 1, "Остановил.") || !hasText(texts, 2, "Остановил.") {
		t.Fatalf("тексты %+v", texts)
	}
}

func TestConfirmGoesToTheRightChat(t *testing.T) {
	b, api, run := newTestBot(t)
	b.confirmFor = 400 * time.Millisecond
	run.fn = func(ctx context.Context, req agent.Request) (agent.Result, error) {
		ok, err := req.Confirm(ctx, "db_stop_bot", nil)
		if err != nil {
			return agent.Result{}, err
		}
		word := "нет"
		if ok {
			word = "да"
		}
		return agent.Result{
			Reply:      word + " " + req.UserText,
			Transcript: []openrouter.Message{{Role: "user", Content: req.UserText}},
		}, nil
	}
	ctx := context.Background()
	for _, id := range []int64{1, 2} {
		if err := b.store.SaveToken(ctx, id, fmt.Sprintf("mcp_user_%d_xx", id)); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		b.process(textUpdate(1, "задача 1"))
	}()
	go func() {
		defer wg.Done()
		b.process(textUpdate(2, "задача 2"))
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if countText(api.snapshot(), "Подтвердить вызов db_stop_bot?") >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if countText(api.snapshot(), "Подтвердить вызов db_stop_bot?") < 2 {
		t.Fatal("оба чата должны спросить подтверждение")
	}
	b.process(callbackUpdate(2, "no:agent"))
	b.process(callbackUpdate(1, "yes:agent"))
	wait(t, &wg)
	texts := api.snapshot()
	if !hasText(texts, 1, "да задача 1") {
		t.Fatal("первый чат не подтвердил свой вызов")
	}
	if !hasText(texts, 2, "нет задача 2") {
		t.Fatal("второй чат получил чужое «да»")
	}
}

func TestRunAgentDeletesStatusInsteadOfGotovo(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "success", want: "ответ агента"},
		{name: "cancel", err: context.Canceled, want: "Остановил."},
		{name: "unauthorized", err: mcp.ErrUnauthorized, want: "Токен отклонён. Откройте «Аккаунт» и задайте новый."},
		{name: "error", err: errors.New("boom"), want: "Не удалось получить ответ: boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, api, run := newTestBot(t)
			if err := b.store.SaveToken(context.Background(), 7, "mcp_user_7_xx"); err != nil {
				t.Fatal(err)
			}
			run.fn = func(ctx context.Context, req agent.Request) (agent.Result, error) {
				if req.OnTool != nil {
					req.OnTool("db_list_bots")
				}
				if tc.err != nil {
					return agent.Result{}, tc.err
				}
				return agent.Result{Reply: "ответ агента"}, nil
			}
			b.process(textUpdate(7, "старт"))
			lines := api.snapshot()
			if hasText(lines, 7, "Готово") {
				t.Fatalf("осталось Готово: %+v", lines)
			}
			if !hasText(lines, 7, "Думаю…") {
				t.Fatalf("пропал статус: %+v", lines)
			}
			if !hasText(lines, 7, "Вызываю db_list_bots") {
				t.Fatalf("пропал статус инструмента: %+v", lines)
			}
			if !hasText(lines, 7, tc.want) {
				t.Fatalf("нет ответа %q в %+v", tc.want, lines)
			}
			api.mu.Lock()
			deleted := append([]int(nil), api.deleted...)
			api.mu.Unlock()
			if len(deleted) != 1 || deleted[0] == 0 {
				t.Fatalf("delete %+v", deleted)
			}
		})
	}
}

func TestRunAgentDeleteFailureDoesNotLeaveGotovo(t *testing.T) {
	b, api, run := newTestBot(t)
	api.deleteErr = errors.New("delete failed")
	if err := b.store.SaveToken(context.Background(), 7, "mcp_user_7_xx"); err != nil {
		t.Fatal(err)
	}
	run.fn = func(ctx context.Context, req agent.Request) (agent.Result, error) {
		req.OnTool("db_list_bots")
		return agent.Result{Reply: "ответ агента"}, nil
	}
	b.process(textUpdate(7, "старт"))
	lines := api.snapshot()
	if hasText(lines, 7, "Готово") {
		t.Fatalf("при ошибке delete осталось Готово: %+v", lines)
	}
	if !hasText(lines, 7, "Вызываю db_list_bots") {
		t.Fatalf("пропал последний статус: %+v", lines)
	}
	if !hasText(lines, 7, "ответ агента") {
		t.Fatalf("ответ не ушёл: %+v", lines)
	}
}

type scriptRunner struct {
	fn func(ctx context.Context, req agent.Request) (agent.Result, error)
}

func (s *scriptRunner) Run(ctx context.Context, req agent.Request) (agent.Result, error) {
	return s.fn(ctx, req)
}

type sentLine struct {
	chat int64
	text string
}

type fakeAPI struct {
	mu        sync.Mutex
	lines     []sentLine
	next      int
	deleted   []int
	deleteErr error
}

func (f *fakeAPI) GetMe(context.Context) (*telego.User, error) {
	return &telego.User{ID: 1, IsBot: true, Username: "builder"}, nil
}

func (f *fakeAPI) SetMyCommands(context.Context, *telego.SetMyCommandsParams) error { return nil }

func (f *fakeAPI) UpdatesViaLongPolling(context.Context, *telego.GetUpdatesParams, ...telego.LongPollingOption) (<-chan telego.Update, error) {
	return nil, nil
}

func (f *fakeAPI) AnswerCallbackQuery(context.Context, *telego.AnswerCallbackQueryParams) error {
	return nil
}

func (f *fakeAPI) SendMessage(_ context.Context, params *telego.SendMessageParams) (*telego.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	f.lines = append(f.lines, sentLine{chat: params.ChatID.ID, text: params.Text})
	return &telego.Message{MessageID: f.next, Chat: telego.Chat{ID: params.ChatID.ID}, Text: params.Text}, nil
}

func (f *fakeAPI) EditMessageText(_ context.Context, params *telego.EditMessageTextParams) (*telego.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lines = append(f.lines, sentLine{chat: params.ChatID.ID, text: params.Text})
	return &telego.Message{MessageID: params.MessageID, Text: params.Text}, nil
}

func (f *fakeAPI) SendRichMessage(_ context.Context, params *RichSendParams) (*telego.Message, error) {
	text := params.HTML
	if text == "" {
		text = params.Markdown
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	f.lines = append(f.lines, sentLine{chat: params.ChatID, text: text})
	return &telego.Message{MessageID: f.next, Chat: telego.Chat{ID: params.ChatID}, Text: text}, nil
}

func (f *fakeAPI) EditRichMessage(_ context.Context, params *RichEditParams) (*telego.Message, error) {
	text := params.HTML
	if text == "" {
		text = params.Markdown
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lines = append(f.lines, sentLine{chat: params.ChatID, text: text})
	return &telego.Message{MessageID: params.MessageID, Text: text}, nil
}

func (f *fakeAPI) DeleteMessage(_ context.Context, params *telego.DeleteMessageParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, params.MessageID)
	return f.deleteErr
}

func (f *fakeAPI) snapshot() []sentLine {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]sentLine, len(f.lines))
	copy(out, f.lines)
	return out
}

func newTestBot(t *testing.T) (*Bot, *fakeAPI, *scriptRunner) {
	t.Helper()
	st, err := store.Open("sqlite://"+t.TempDir()+"/agent.db", 20)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	log, err := logger.New("ERROR", "")
	if err != nil {
		t.Fatal(err)
	}
	api := &fakeAPI{}
	run := &scriptRunner{}
	b := &Bot{
		api:    api,
		cfg:    &config.Config{},
		log:    log,
		store:  st,
		runner: run,
		root:   context.Background(),
		locks:  map[int64]*sync.Mutex{},
		sess:   map[int64]*session{},
	}
	return b, api, run
}

func textUpdate(id int64, text string) telego.Update {
	return telego.Update{Message: &telego.Message{
		From: &telego.User{ID: id},
		Chat: telego.Chat{ID: id},
		Text: text,
	}}
}

func callbackUpdate(id int64, data string) telego.Update {
	msg := &telego.Message{Chat: telego.Chat{ID: id}}
	return telego.Update{CallbackQuery: &telego.CallbackQuery{
		ID:      fmt.Sprintf("cb-%d-%s", id, data),
		From:    telego.User{ID: id},
		Data:    data,
		Message: msg,
	}}
}

func hasText(lines []sentLine, chat int64, text string) bool {
	for _, line := range lines {
		if line.chat == chat && line.text == text {
			return true
		}
	}
	return false
}

func countText(lines []sentLine, text string) int {
	n := 0
	for _, line := range lines {
		if line.text == text {
			n++
		}
	}
	return n
}

func wait(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("чаты не завершились")
	}
}
