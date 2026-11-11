# Record at the client interface with strict replay

Status: accepted (generated, pending human review)

## Context

LLM-dependent tests are slow, costly, and non-deterministic. HTTP cassettes are fragile because they key on transport-level details instead of the application-level request.

## Decision

Record LLM client calls to a JSONL session file at the `Client` interface, including a hash of the canonical request. Replay strictly in order, consuming entries only on exact hash match and reporting the first differing field as a typed error.

## Consequences

Tests fail precisely when the request changes, with field-level detail. The replay file is human-readable and versionable. Size limits and integrity checks prevent accidental corruption and resource exhaustion.
