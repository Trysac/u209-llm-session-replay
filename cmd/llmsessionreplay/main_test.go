package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	llmsessionreplay "github.com/Trysac/u209-llm-session-replay"
)

type fakeClient struct {
	resp llmsessionreplay.Response
	err  error
}

func (f fakeClient) Complete(ctx context.Context, req llmsessionreplay.Request) (llmsessionreplay.Response, error) {
	return f.resp, f.err
}

func TestRunValidFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	inner := fakeClient{resp: llmsessionreplay.Response{Text: "hello"}}
	rec := llmsessionreplay.NewRecorder(inner, f)
	req1 := llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "hi"}}, MaxTokens: 16}
	if _, err := rec.Complete(context.Background(), req1); err != nil {
		t.Fatal(err)
	}
	req2 := llmsessionreplay.Request{Model: "m2", Messages: []llmsessionreplay.Message{{Role: "user", Content: "hi2"}}, MaxTokens: 20}
	if _, err := rec.Complete(context.Background(), req2); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"verify", f.Name()}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if stdout.String() != "ok 2 entries\n" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "ok 2 entries\n")
	}
}

func TestRunTamperedFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "tampered.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	content := `{"seq":1,"request_hash":"wrong","request":{"model":"m","messages":[{"role":"user","content":"hi"}],"max_tokens":16},"response":{"text":"hello"}}
`
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"verify", f.Name()}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr.String(), "line 1: hash mismatch") {
		t.Fatalf("stderr = %q, want prefix 'line 1: hash mismatch'", stderr.String())
	}
}

func TestRunMissingArg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"verify"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
}

func TestRunUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"bogus", "x"}, &stdout, &stderr); code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
}

func TestRunOpenError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	path := filepath.Join(t.TempDir(), "missing.jsonl")
	if code := Run([]string{"verify", path}, &stdout, &stderr); code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
}
