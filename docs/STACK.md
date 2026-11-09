# Stack

## Used

| Tool | Why |
|---|---|
| Go >= 1.22 | Supported language version for the library and CLI. |
| Go modules | Module path `github.com/Trysac/u209-llm-session-replay`; no third-party dependencies. |
| gofmt + go vet | Canonical formatting and static checks without extra configuration. |
| go test (race detector, cover) | Built-in tests, race detection and coverage. |
| Standard library only | `allowed_dependencies` is empty; the library and CLI use stdlib packages. |

## Discarded

| Tool | Why not |
|---|---|
| External dependencies | The specification allows none. |
| testify | The standard `testing` package keeps the module dependency-free. |
| Makefile / shell scripts for gates | Not portable to Windows; gates can be a stdlib Go program. |
