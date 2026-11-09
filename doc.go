// Package llmsessionreplay is a small, dependency-free Go library: Record LLM client calls to a JSONL session file and replay them strictly in order for deterministic Go tests, with request hashes, field-level mismatch errors and integrity checks
//
// The public API is every exported identifier of this root package. Functions are pure: they do
// no I/O, keep no global mutable state and report invalid input through returned errors.
package llmsessionreplay
