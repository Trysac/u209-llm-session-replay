# Security notes

## Threat model

### Defended

- Request changes: `request_hash` is the lowercase hex SHA-256 of canonical request JSON produced with `json.Encoder` and `SetEscapeHTML(false)`, with the trailing newline removed. Replay compares the hash and reports the first differing field.
- Record-level integrity: `NewReplayer` rejects invalid JSON, non-consecutive `seq`, hash mismatch, blank lines, lines longer than `1_048_576` bytes, and more than `100_000` entries with typed errors (`ErrCorrupt`, `ErrTooLarge`).
- Accidental prompt leakage through library errors: library-generated errors do not include request message contents. `RecordedError` preserves the inner client's error text.
- Context cancellation: `Complete` checks `ctx.Err()` first and consumes nothing on cancellation.
- Dependencies: standard library only.

### Not defended

- Authenticity: hashes are not signatures. A party that can rewrite the session file can recompute `request_hash` and forge entries. The format does not authenticate the recorder or the file.
- Confidentiality: the JSONL file stores requests and responses in plaintext. There is no encryption and no redaction. Do not record credentials or secrets in prompts; requests carry no credentials by design.
- Untrusted files: replay validates structure and hashes but will return whatever responses or errors a well-formed crafted file contains.
- Total resource use: the reader enforces per-line (`1_048_576` bytes) and entry-count (`100_000`) limits, but `NewReplayer` reads the whole session into memory on construction.
- Transport-level attacks: the library does not record or replay HTTP headers, bodies, streaming chunks, or fuzzy matches; those are out of scope.

## Limits

- Maximum line length: `1_048_576` bytes.
- Maximum entries: `100_000`.
- No blank lines.
- Call order must match the recorded order; a mutex protects concurrent state, but the caller is responsible for replay order.

## Publishing

Releases are git tags served through the Go module proxy and checksum database. No registry token is stored in the repository.

To report a vulnerability, see [SECURITY.md](../SECURITY.md).
