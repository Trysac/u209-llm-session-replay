# llm-session-replay

Record LLM client calls to a JSONL session file and replay them strictly in order for deterministic Go tests, with request hashes, field-level mismatch errors and integrity checks.

## Installation

```
go get github.com/Trysac/u209-llm-session-replay
```

## Usage

```go
package main
import ("bytes"; "context"; "fmt"; r "github.com/Trysac/u209-llm-session-replay")
type c struct{}
func (c) Complete(context.Context, r.Request) (r.Response, error) { return r.Response{Text: "hello"}, nil }
func main() {
ctx, buf := context.Background(), &bytes.Buffer{}
req := r.Request{Model: "m", Messages: []r.Message{{Role: "user", Content: "hi"}}, MaxTokens: 16}
rec := r.NewRecorder(c{}, buf); _, _ = rec.Complete(ctx, req)
rp, _ := r.NewReplayer(buf); resp, _ := rp.Complete(ctx, req)
fmt.Println(resp.Text) }
```

A complete example is in docs/demo.md.

## Public API

- `Message{Role string; Content string}`
- `Request{Model string; Messages []Message; MaxTokens int; Temperature *float64}`
- `Response{Text string; StopReason string; InputTokens int; OutputTokens int}`
- `Client interface { Complete(ctx context.Context, req Request) (Response, error) }`
- `NewRecorder(inner Client, w io.Writer) *Recorder` implements `Client`. It writes one JSON object per line with `seq` starting at 1, `request_hash`, `request`, and either `response` or `error`; it returns the inner result unchanged, and write errors are returned wrapped after the call.
- `NewReplayer(r io.Reader) (*Replayer, error)` implements `Client` and has `Remaining() int` and `Done() error`. It reads the full session on construction and consumes entries strictly in order.
- The request hash is lowercase hex SHA-256 of canonical request JSON produced with `json.Encoder` and `SetEscapeHTML(false)`, with the trailing newline removed. `Temperature` nil and `0` produce different hashes.

## Errors, limits and options

- Sentinel errors: `ErrMismatch`, `ErrExhausted`, `ErrCorrupt`, `ErrTooLarge`, `ErrUnconsumed`.
- `ReplayError{Code string; Seq int; Field string; Line int}`: `Code` is `mismatch`, `exhausted`, `corrupt`, `too_large`, or `unconsumed`; `Unwrap` returns the matching sentinel. A mismatch names the first differing field in this order: `model`, `max_tokens`, `temperature`, `messages` length, `messages[i].role`, `messages[i].content`. A mismatch does not consume the entry.
- `RecordedError{Message string}` is returned when replaying a recorded client error; `errors.As(err, &RecordedError{})` works.
- `Complete` checks `ctx.Err()` first. Exhausted replay returns `ErrExhausted` and consumes nothing. After partial replay, `Done` returns `ErrUnconsumed` with message `llmsessionreplay: N recorded calls not replayed`.
- Limits: lines longer than `1_048_576` bytes return `ErrTooLarge`; more than `100_000` entries return `ErrTooLarge`; blank lines are not allowed; malformed JSON, hash mismatch and non-consecutive `seq` return `ErrCorrupt` with a `line N:` prefix.
- Concurrency: `Recorder` and `Replayer` protect state with a mutex; call order is the caller's responsibility.
- Options: none. The only optional request field is `Temperature` (`*float64`).

## CLI

```
go run ./cmd/llmsessionreplay verify session.jsonl
```

`Run(args []string, stdout, stderr io.Writer) int` implements subcommand `verify <file>`. Exit code 0 when valid, 1 when corrupt, 2 on usage or I/O errors. A valid file prints `ok N entries`; a hash mismatch prints `line 1: hash mismatch` to stderr.

## Why not HTTP cassettes

HTTP cassettes key on headers and bodies at the HTTP layer. This library records at the `Client` interface, so replay fails on the exact model request field that changed, such as `messages[0].content`, instead of on transport details.

## Documentation

- [Architecture](docs/ARCHITECTURE.md)
- [Security](docs/SECURITY.md)
- [Testing](docs/TESTING.md)
- [Project](docs/PROJECT.md)
