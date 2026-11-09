# Testing

## Running

```sh
go test -cover .
go test -race .
go run ./scripts/check
```

## Policy

Tests use the standard `testing` package and package `llmsessionreplay_test`; they exercise the public API. They are deterministic and do not require network access or external test dependencies. The race detector runs when cgo and a C compiler are available.

## What the tests cover

The tests are in `*_test.go` files next to the package. The specification does not name individual test files; the suite covers these observable behaviors:

- Recorder writes one line with the documented request hash for `Request{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 16}` and returns the fake's full `Response{Text: "hello", StopReason: "end", InputTokens: 3, OutputTokens: 1}`.
- Replayer returns the same response without calling an inner client; a second call gives `ErrExhausted` with `ReplayError` code `exhausted` and consumes nothing.
- Replaying changed message content gives `ErrMismatch`, `ReplayError` with `Seq: 1` and `Field: "messages[0].content"`, and `Remaining()` stays 1.
- An inner client error `rate limited` is recorded as an error line with no response; replay returns that error text and `errors.As(err, &RecordedError{})` is true.
- `NewReplayer` on `not json` gives `ErrCorrupt` with a `line 1:` prefix; a hash mismatch or non-consecutive `seq` also gives `ErrCorrupt`; a `1_048_577`-byte line gives `ErrTooLarge`.
- After consuming 1 of 2 entries, `Done()` gives `ErrUnconsumed` with message `llmsessionreplay: 1 recorded calls not replayed`; an already cancelled context returns `ctx.Err()` and consumes nothing.
- `Run([]string{"verify", "session.jsonl"}, stdout, stderr)` prints `ok 2 entries` and returns 0 for a valid file; a tampered file returns 1 and prints `line 1: hash mismatch` to stderr; a missing file argument returns 2.
- Edge cases: `Temperature` nil versus `0` have different hashes; unicode and `<` in content are not HTML-escaped; an empty session file makes `Done()` return nil; two goroutines replay under `go test -race`.

## Tiers

The tests run under `go test` locally and in CI. The suite is deterministic and does not require a separate conformance setup from the specification.

## Layout

`<file>_test.go` sits next to the file it tests. Fixtures and golden files go in `testdata/`.
