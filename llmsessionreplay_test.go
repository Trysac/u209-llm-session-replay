package llmsessionreplay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	llmsessionreplay "github.com/Trysac/u209-llm-session-replay"
)

// fakeClient is a simple Client for testing.
type fakeClient struct {
	resp llmsessionreplay.Response
	err  error
}

func (f fakeClient) Complete(ctx context.Context, req llmsessionreplay.Request) (llmsessionreplay.Response, error) {
	return f.resp, f.err
}

func TestRecorderAndReplayBasic(t *testing.T) {
	var buf bytes.Buffer
	inner := fakeClient{resp: llmsessionreplay.Response{Text: "hello", StopReason: "end", InputTokens: 3, OutputTokens: 1}}
	rec := llmsessionreplay.NewRecorder(inner, &buf)

	ctx := context.Background()
	req := llmsessionreplay.Request{
		Model:     "m",
		Messages:  []llmsessionreplay.Message{{Role: "user", Content: "hi"}},
		MaxTokens: 16,
	}
	resp, err := rec.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if resp != inner.resp {
		t.Fatalf("Complete response = %+v, want %+v", resp, inner.resp)
	}

	line := buf.String()
	if !strings.HasSuffix(line, "\n") {
		t.Fatal("recorded line missing newline")
	}
	line = strings.TrimSuffix(line, "\n")

	var entry struct {
		Seq         int                        `json:"seq"`
		RequestHash string                     `json:"request_hash"`
		Request     llmsessionreplay.Request   `json:"request"`
		Response    *llmsessionreplay.Response `json:"response"`
		Error       string                     `json:"error"`
	}
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("unmarshal line: %v", err)
	}
	if entry.Seq != 1 {
		t.Fatalf("seq = %d, want 1", entry.Seq)
	}
	wantHash := "b0426255d75d8f356c60baa175ae61c3d46df2e29162dceeee745d46a2166681"
	if entry.RequestHash != wantHash {
		t.Fatalf("request_hash = %s, want %s", entry.RequestHash, wantHash)
	}
	if entry.Response == nil || *entry.Response != inner.resp {
		t.Fatalf("recorded response = %+v, want %+v", entry.Response, inner.resp)
	}

	// Now replay from the buffer.
	rp, err := llmsessionreplay.NewReplayer(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("NewReplayer error: %v", err)
	}
	gotResp, err := rp.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Replay Complete error: %v", err)
	}
	if gotResp != inner.resp {
		t.Fatalf("replay response = %+v, want %+v", gotResp, inner.resp)
	}
	if rp.Remaining() != 0 {
		t.Fatalf("Remaining = %d, want 0", rp.Remaining())
	}
	if err := rp.Done(); err != nil {
		t.Fatalf("Done after full replay returned error: %v", err)
	}
}

func TestReplayerExhausted(t *testing.T) {
	var buf bytes.Buffer
	inner := fakeClient{resp: llmsessionreplay.Response{Text: "hello"}}
	rec := llmsessionreplay.NewRecorder(inner, &buf)
	req := llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "hi"}}}
	if _, err := rec.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	rp, err := llmsessionreplay.NewReplayer(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rp.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	_, err = rp.Complete(context.Background(), req)
	if !errors.Is(err, llmsessionreplay.ErrExhausted) {
		t.Fatalf("second replay error = %v, want ErrExhausted", err)
	}
	var re *llmsessionreplay.ReplayError
	if !errors.As(err, &re) || re.Code != "exhausted" {
		t.Fatalf("error is not ReplayError with Code exhausted: %#v", err)
	}
}

func TestMismatchFieldAndNotConsumed(t *testing.T) {
	var buf bytes.Buffer
	inner := fakeClient{resp: llmsessionreplay.Response{Text: "hello"}}
	rec := llmsessionreplay.NewRecorder(inner, &buf)
	req := llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "hi"}}}
	if _, err := rec.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	rp, err := llmsessionreplay.NewReplayer(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	badReq := req
	badReq.Messages[0].Content = "bye"
	_, err = rp.Complete(context.Background(), badReq)
	if !errors.Is(err, llmsessionreplay.ErrMismatch) {
		t.Fatalf("mismatch error = %v, want ErrMismatch", err)
	}
	var re *llmsessionreplay.ReplayError
	if !errors.As(err, &re) {
		t.Fatalf("error is not ReplayError: %v", err)
	}
	if re.Seq != 1 || re.Field != "messages[0].content" {
		t.Fatalf("ReplayError fields = seq %d field %q, want seq 1 field %q", re.Seq, re.Field, "messages[0].content")
	}
	if rp.Remaining() != 1 {
		t.Fatalf("Remaining after mismatch = %d, want 1", rp.Remaining())
	}
}

func TestRecordedError(t *testing.T) {
	var buf bytes.Buffer
	inner := fakeClient{err: errors.New("rate limited")}
	rec := llmsessionreplay.NewRecorder(inner, &buf)
	req := llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "hi"}}}
	if _, err := rec.Complete(context.Background(), req); err == nil {
		t.Fatal("expected error from inner client")
	}

	// The recorded line must carry the error and no response.
	if got := strings.Count(buf.String(), "\n"); got != 1 {
		t.Fatalf("recorded newline count = %d, want 1", got)
	}
	var line struct {
		Error    string                     `json:"error"`
		Response *llmsessionreplay.Response `json:"response"`
	}
	if err := json.Unmarshal(bytes.TrimSuffix(buf.Bytes(), []byte("\n")), &line); err != nil {
		t.Fatalf("unmarshal recorded line: %v", err)
	}
	if line.Error != "rate limited" {
		t.Fatalf("recorded error = %q, want %q", line.Error, "rate limited")
	}
	if line.Response != nil {
		t.Fatalf("recorded response = %+v, want nil", line.Response)
	}

	rp, err := llmsessionreplay.NewReplayer(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	_, err = rp.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "rate limited" {
		t.Fatalf("error message = %q, want %q", err.Error(), "rate limited")
	}
	var recErr *llmsessionreplay.RecordedError
	if !errors.As(err, &recErr) {
		t.Fatalf("error is not RecordedError: %T", err)
	}
}

func TestNewReplayerCorrupt(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantErr    error
		wantPrefix string
	}{
		{name: "not json", line: "not json", wantErr: llmsessionreplay.ErrCorrupt, wantPrefix: "line 1:"},
		{name: "hash mismatch", line: `{"seq":1,"request_hash":"bad","request":{"model":"m","messages":[{"role":"user","content":"hi"}],"max_tokens":16},"response":{"text":"hello"}}`, wantErr: llmsessionreplay.ErrCorrupt, wantPrefix: "line 1:"},
		{name: "seq non-consecutive", line: `{"seq":2,"request_hash":"b0426255d75d8f356c60baa175ae61c3d46df2e29162dceeee745d46a2166681","request":{"model":"m","messages":[{"role":"user","content":"hi"}],"max_tokens":16},"response":{"text":"hello"}}`, wantErr: llmsessionreplay.ErrCorrupt, wantPrefix: "line 1:"},
		{name: "blank line", line: "\n", wantErr: llmsessionreplay.ErrCorrupt, wantPrefix: "line 1:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := llmsessionreplay.NewReplayer(strings.NewReader(tt.line))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewReplayer error = %v, want %v", err, tt.wantErr)
			}
			if !strings.HasPrefix(err.Error(), tt.wantPrefix) {
				t.Fatalf("error message = %q, does not have prefix %q", err.Error(), tt.wantPrefix)
			}
		})
	}
}

func TestNewReplayerTooLong(t *testing.T) {
	longLine := strings.Repeat("a", 1_048_577)
	_, err := llmsessionreplay.NewReplayer(strings.NewReader(longLine))
	if !errors.Is(err, llmsessionreplay.ErrTooLarge) {
		t.Fatalf("error = %v, want ErrTooLarge", err)
	}
}

func TestDoneUnconsumed(t *testing.T) {
	var buf bytes.Buffer
	inner := fakeClient{resp: llmsessionreplay.Response{Text: "one"}}
	rec := llmsessionreplay.NewRecorder(inner, &buf)
	req1 := llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "hi1"}}}
	if _, err := rec.Complete(context.Background(), req1); err != nil {
		t.Fatal(err)
	}
	req2 := llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "hi2"}}}
	if _, err := rec.Complete(context.Background(), req2); err != nil {
		t.Fatal(err)
	}

	rp, err := llmsessionreplay.NewReplayer(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	// consume one
	if _, err := rp.Complete(context.Background(), req1); err != nil {
		t.Fatal(err)
	}
	err = rp.Done()
	if !errors.Is(err, llmsessionreplay.ErrUnconsumed) {
		t.Fatalf("Done error = %v, want ErrUnconsumed", err)
	}
	if err.Error() != "llmsessionreplay: 1 recorded calls not replayed" {
		t.Fatalf("Done error message = %q, want %q", err.Error(), "llmsessionreplay: 1 recorded calls not replayed")
	}
}

func TestTemperatureNilVersusZeroHash(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	inner := fakeClient{resp: llmsessionreplay.Response{}}
	rec1 := llmsessionreplay.NewRecorder(inner, &buf1)
	rec2 := llmsessionreplay.NewRecorder(inner, &buf2)

	reqNil := llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "hi"}}}
	temp0 := 0.0
	reqZero := reqNil
	reqZero.Temperature = &temp0

	if _, err := rec1.Complete(context.Background(), reqNil); err != nil {
		t.Fatal(err)
	}
	if _, err := rec2.Complete(context.Background(), reqZero); err != nil {
		t.Fatal(err)
	}

	// Extract hashes from recorded lines.
	getHash := func(buf bytes.Buffer) string {
		var e struct {
			RequestHash string `json:"request_hash"`
		}
		if err := json.Unmarshal(bytes.TrimRight(buf.Bytes(), "\n"), &e); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return e.RequestHash
	}
	hashNil := getHash(buf1)
	hashZero := getHash(buf2)
	if hashNil == hashZero {
		t.Fatal("hashes for nil and zero temperature should differ")
	}
}

func TestUnicodeAndHTMLNotEscaped(t *testing.T) {
	var buf bytes.Buffer
	inner := fakeClient{resp: llmsessionreplay.Response{}}
	rec := llmsessionreplay.NewRecorder(inner, &buf)
	req := llmsessionreplay.Request{
		Model:    "m",
		Messages: []llmsessionreplay.Message{{Role: "user", Content: "héllo <world>"}},
	}
	if _, err := rec.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	var e struct {
		Request llmsessionreplay.Request `json:"request"`
	}
	if err := json.Unmarshal(bytes.TrimRight(buf.Bytes(), "\n"), &e); err != nil {
		t.Fatal(err)
	}
	if e.Request.Messages[0].Content != "héllo <world>" {
		t.Fatalf("content = %q, want %q", e.Request.Messages[0].Content, "héllo <world>")
	}
	// Ensure no HTML escaping happened: the raw JSON should contain the literal characters.
	if !strings.Contains(buf.String(), "héllo <world>") {
		t.Fatal("raw JSON does not contain unescaped unicode and HTML")
	}
}

func TestConcurrentReplay(t *testing.T) {
	var buf bytes.Buffer
	inner := fakeClient{resp: llmsessionreplay.Response{Text: "hello"}}
	rec := llmsessionreplay.NewRecorder(inner, &buf)
	req := llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "hi"}}}
	if _, err := rec.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	rp, err := llmsessionreplay.NewReplayer(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = rp.Complete(context.Background(), req)
			_ = rp.Remaining()
			_ = rp.Done()
		}()
	}
	wg.Wait()
}

func TestReplayerCtxCancel(t *testing.T) {
	var buf bytes.Buffer
	inner := fakeClient{resp: llmsessionreplay.Response{Text: "hello"}}
	rec := llmsessionreplay.NewRecorder(inner, &buf)
	req := llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "hi"}}}
	if _, err := rec.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	rp, err := llmsessionreplay.NewReplayer(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = rp.Complete(ctx, req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if rp.Remaining() != 1 {
		t.Fatalf("Remaining after canceled call = %d, want 1", rp.Remaining())
	}
}

func TestMismatchFieldOrder(t *testing.T) {
	var buf bytes.Buffer
	temp := 0.5
	inner := fakeClient{resp: llmsessionreplay.Response{Text: "hello"}}
	rec := llmsessionreplay.NewRecorder(inner, &buf)
	recorded := llmsessionreplay.Request{
		Model:       "m",
		Messages:    []llmsessionreplay.Message{{Role: "user", Content: "hi"}, {Role: "assistant", Content: "ok"}},
		MaxTokens:   16,
		Temperature: &temp,
	}
	if _, err := rec.Complete(context.Background(), recorded); err != nil {
		t.Fatal(err)
	}
	rp, err := llmsessionreplay.NewReplayer(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}

	otherTemp := 0.7
	tests := []struct {
		name  string
		req   llmsessionreplay.Request
		field string
	}{
		{"model", llmsessionreplay.Request{Model: "m2", Messages: recorded.Messages, MaxTokens: 16, Temperature: &temp}, "model"},
		{"max_tokens", llmsessionreplay.Request{Model: "m", Messages: recorded.Messages, MaxTokens: 17, Temperature: &temp}, "max_tokens"},
		{"temperature nil", llmsessionreplay.Request{Model: "m", Messages: recorded.Messages, MaxTokens: 16}, "temperature"},
		{"temperature value", llmsessionreplay.Request{Model: "m", Messages: recorded.Messages, MaxTokens: 16, Temperature: &otherTemp}, "temperature"},
		{"messages length", llmsessionreplay.Request{Model: "m", Messages: recorded.Messages[:1], MaxTokens: 16, Temperature: &temp}, "messages"},
		{"role", llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "hi"}, {Role: "tool", Content: "ok"}}, MaxTokens: 16, Temperature: &temp}, "messages[1].role"},
		{"content", llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "hi"}, {Role: "assistant", Content: "no"}}, MaxTokens: 16, Temperature: &temp}, "messages[1].content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rp.Complete(context.Background(), tt.req)
			if !errors.Is(err, llmsessionreplay.ErrMismatch) {
				t.Fatalf("error = %v, want ErrMismatch", err)
			}
			var re *llmsessionreplay.ReplayError
			if !errors.As(err, &re) || re.Field != tt.field {
				t.Fatalf("field = %v, want %q", err, tt.field)
			}
			if rp.Remaining() != 1 {
				t.Fatalf("Remaining = %d, want 1", rp.Remaining())
			}
		})
	}
}

func TestEmptySession(t *testing.T) {
	rp, err := llmsessionreplay.NewReplayer(strings.NewReader(""))
	if err != nil {
		t.Fatalf("NewReplayer error: %v", err)
	}
	if rp.Remaining() != 0 {
		t.Fatalf("Remaining = %d, want 0", rp.Remaining())
	}
	if err := rp.Done(); err != nil {
		t.Fatalf("Done = %v, want nil", err)
	}
}

func TestRecorderWriteError(t *testing.T) {
	inner := fakeClient{resp: llmsessionreplay.Response{Text: "hello"}}
	w := failingWriter{}
	rec := llmsessionreplay.NewRecorder(inner, w)
	req := llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "hi"}}}
	_, err := rec.Complete(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "writing record") {
		t.Fatalf("expected writing record error, got %v", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) { return 0, errors.New("write failed") }
