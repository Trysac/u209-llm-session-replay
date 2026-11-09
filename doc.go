// Package llmsessionreplay records LLM client calls to a JSONL session file
// and replays them strictly in order for deterministic Go tests.
//
// Use Recorder to wrap a real Client and record interactions. Use Replayer
// to read the recorded file and replay responses for matching requests.
// Mismatch errors identify the first differing field.
package llmsessionreplay
