# Architecture

## Recording

`Recorder` wraps a `Client` and writes one JSON object per call to the provided `io.Writer`. Each line contains the sequence number, request hash, full request, and either a response or an error message. `seq` starts at 1 and increases by 1. The hash is the lowercase hex SHA-256 of the canonical JSON encoding produced with `json.Encoder` and `SetEscapeHTML(false)`, with the trailing newline removed. Recorder writes each line under a mutex and returns the inner result unchanged; write errors are returned wrapped after the call.

## Replaying

`Replayer` reads the entire file on construction, validating structure, sequence numbers, and hashes. During `Complete`, it checks `ctx.Err()`, ensures an entry remains, and compares the hash of the incoming request. On mismatch, it identifies the first differing field using this order: model, max_tokens, temperature, messages length, then each message's role and content. A mismatch does not consume the entry. Errors are typed: `ReplayError` with a machine-readable code and sentinel support via `errors.Is`.

`Complete` returns recorded client errors as `RecordedError`. `Remaining` reports unconsumed entries, and `Done` returns `ErrUnconsumed` when entries remain.

## Concurrency

Both `Recorder` and `Replayer` protect their state with a mutex. Recorder serializes writes and ensures unique sequence numbers; Replayer serializes access to the entry queue. Callers are responsible for replaying calls in the same order.

## Limits

- Lines up to `1_048_576` bytes; longer lines return `ErrTooLarge`.
- Maximum `100_000` entries; more entries return `ErrTooLarge`.
- No blank lines.
- Malformed JSON, hash mismatch, and non-consecutive `seq` return `ErrCorrupt` with a `line N:` prefix.
- Library-generated error messages never contain message contents.
- The CLI is `cmd/llmsessionreplay`; `Run(args []string, stdout, stderr io.Writer) int` handles `verify <file>` with exit codes 0 for valid, 1 for corrupt, and 2 for usage or I/O errors.
