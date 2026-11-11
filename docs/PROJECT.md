# Project record

## Objective

Record LLM client calls to a JSONL session file and replay them strictly in order for deterministic Go tests, with request hashes, field-level mismatch errors and integrity checks.

## Scope

In scope:

- The `llmsessionreplay` library with `Message`, `Request`, `Response`, `Client`, `NewRecorder`, and `NewReplayer` as documented in [README.md](../README.md).
- JSONL session recording, strict ordered replay, request hashing, field-level mismatch reporting, and integrity checks.
- The `cmd/llmsessionreplay` CLI with `Run(args []string, stdout, stderr io.Writer) int` and subcommand `verify <file>`.
- Standard library only; `allowed_dependencies` is empty.

Out of scope:

- HTTP-level recording.
- Streaming responses.
- Fuzzy matching.
- Redaction; requests carry no credentials.

## Timeline

| Date | Event |
|---|---|
| Not recorded | The specification does not include dated milestones. |

## Effort metrics

| Metric | Value |
|---|---|
| Tokens (input / output) | 1353537 / 93064 |
| Hours (human) | pending measurement |
| Cost (USD) | 0.1016 |

## Factory run u209 (measured from API usage)

| Phase | Model | Batch | Input | Output | Cache write | Cache read | Cost (USD) |
|---|---|---|---|---|---|---|---|
| generation | deepseek-v4-pro | yes | 1660 | 18764 | 0 | 3328 | 0.0383 |
| fix-1 | deepseek-flash | no | 24599 | 20678 | 0 | 514688 | 0.0176 |
| fix-2 | deepseek-flash | no | 10973 | 5651 | 0 | 98560 | 0.0053 |
| audit | deepseek-flash | no | 24681 | 21600 | 0 | 342528 | 0.0177 |
| fix-3 | deepseek-flash | no | 11480 | 2076 | 0 | 85760 | 0.0032 |
| threat-model | deepseek-flash | no | 23693 | 9680 | 0 | 207104 | 0.0100 |
| docs | deepseek-flash | no | 4099 | 14615 | 0 | 384 | 0.0094 |

Total: $0.1016. Prices: deepseek line, as measured by its adapter.
