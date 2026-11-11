package llmsessionreplay

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Message represents a single message in an LLM conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request is a completion request sent to an LLM.
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature *float64  `json:"temperature,omitempty"`
}

// Response is a completion response returned by an LLM.
type Response struct {
	Text         string `json:"text"`
	StopReason   string `json:"stop_reason"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

// Client is the interface for LLM completion calls.
type Client interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

// Sentinel errors for replay operations.
var (
	// ErrMismatch indicates the request did not match the recorded call.
	ErrMismatch = errors.New("llmsessionreplay: request mismatch")
	// ErrExhausted indicates no recorded calls remain.
	ErrExhausted = errors.New("llmsessionreplay: no recorded calls left")
	// ErrCorrupt indicates the session file is malformed.
	ErrCorrupt = errors.New("llmsessionreplay: corrupt session file")
	// ErrTooLarge indicates a line exceeded the size limit or too many entries.
	ErrTooLarge = errors.New("llmsessionreplay: line too long or too many entries")
	// ErrUnconsumed indicates recorded calls were not replayed before Done.
	ErrUnconsumed = errors.New("llmsessionreplay: recorded calls not replayed")
)

// ReplayError provides detailed information about a replay failure.
type ReplayError struct {
	// Code is a machine-readable error code: "mismatch", "exhausted", "corrupt", "too_large", "unconsumed".
	Code string
	// Seq is the sequence number of the recorded call (0 if not applicable).
	Seq int
	// Field is the first mismatched field name, if any.
	Field string
	// Line is the line number in the session file (0 if not applicable).
	Line int

	msg   string
	cause error
}

func (e *ReplayError) Error() string { return e.msg }

// Unwrap returns the sentinel error corresponding to Code.
func (e *ReplayError) Unwrap() error { return e.cause }

// RecordedError represents an error that was recorded from the inner client.
type RecordedError struct {
	Message string
}

func (e *RecordedError) Error() string { return e.Message }

// Helper constructors for ReplayError.
func newMismatch(seq int, field string, line int) *ReplayError {
	return &ReplayError{
		Code:  "mismatch",
		Seq:   seq,
		Field: field,
		Line:  line,
		msg:   fmt.Sprintf("llmsessionreplay: mismatch at seq %d field %s (line %d)", seq, field, line),
		cause: ErrMismatch,
	}
}

func newExhausted() *ReplayError {
	return &ReplayError{
		Code:  "exhausted",
		msg:   "llmsessionreplay: no recorded calls left",
		cause: ErrExhausted,
	}
}

func newCorrupt(line int, detail string) *ReplayError {
	return &ReplayError{
		Code:  "corrupt",
		Line:  line,
		msg:   fmt.Sprintf("line %d: %s", line, detail),
		cause: ErrCorrupt,
	}
}

func newTooLarge(line int, detail string) *ReplayError {
	return &ReplayError{
		Code:  "too_large",
		Line:  line,
		msg:   fmt.Sprintf("line %d: %s", line, detail),
		cause: ErrTooLarge,
	}
}

func newUnconsumed(remaining int) *ReplayError {
	return &ReplayError{
		Code:  "unconsumed",
		msg:   fmt.Sprintf("llmsessionreplay: %d recorded calls not replayed", remaining),
		cause: ErrUnconsumed,
	}
}

// Recorder wraps a Client and writes each call to an io.Writer as a JSONL line.
type Recorder struct {
	inner Client
	w     io.Writer
	mu    sync.Mutex
	seq   int
}

// NewRecorder returns a Recorder that delegates to inner and records calls to w.
func NewRecorder(inner Client, w io.Writer) *Recorder {
	return &Recorder{inner: inner, w: w, seq: 1}
}

// Complete calls the inner client, records the interaction, and returns the inner result unchanged.
// If recording fails, the error is wrapped and returned with a zero Response.
func (r *Recorder) Complete(ctx context.Context, req Request) (Response, error) {
	resp, err := r.inner.Complete(ctx, req)

	hash, hashErr := computeRequestHash(req)
	if hashErr != nil {
		return Response{}, fmt.Errorf("llmsessionreplay: computing request hash: %w", hashErr)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry := struct {
		Seq         int       `json:"seq"`
		RequestHash string    `json:"request_hash"`
		Request     Request   `json:"request"`
		Response    *Response `json:"response,omitempty"`
		Error       string    `json:"error,omitempty"`
	}{
		Seq:         r.seq,
		RequestHash: hash,
		Request:     req,
	}
	if err != nil {
		entry.Error = err.Error()
	} else {
		entry.Response = &resp
	}
	r.seq++

	var line bytes.Buffer
	enc := json.NewEncoder(&line)
	enc.SetEscapeHTML(false)
	if marshalErr := enc.Encode(entry); marshalErr != nil {
		return Response{}, fmt.Errorf("llmsessionreplay: encoding record: %w", marshalErr)
	}
	if _, writeErr := r.w.Write(line.Bytes()); writeErr != nil {
		return resp, fmt.Errorf("llmsessionreplay: writing record: %w", writeErr)
	}
	return resp, err
}

// Replayer reads a recorded session and replays calls strictly in order.
type Replayer struct {
	mu      sync.Mutex
	entries []replayEntry
	next    int
}

type replayEntry struct {
	seq          int
	line         int
	requestHash  string
	request      Request
	response     *Response
	errorMessage string
}

// NewReplayer reads the entire session from r, validates it, and returns a Replayer.
// Limits: lines up to 1_048_576 bytes, at most 100_000 entries.
func NewReplayer(r io.Reader) (*Replayer, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1_048_576)

	var entries []replayEntry
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			return nil, newCorrupt(lineNum, "blank line")
		}

		var e struct {
			Seq         int       `json:"seq"`
			RequestHash string    `json:"request_hash"`
			Request     Request   `json:"request"`
			Response    *Response `json:"response,omitempty"`
			Error       string    `json:"error,omitempty"`
		}
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, newCorrupt(lineNum, err.Error())
		}

		expectedSeq := len(entries) + 1
		if e.Seq != expectedSeq {
			return nil, newCorrupt(lineNum, fmt.Sprintf("seq mismatch: got %d, want %d", e.Seq, expectedSeq))
		}

		hash, err := computeRequestHash(e.Request)
		if err != nil {
			return nil, newCorrupt(lineNum, "request hash error: "+err.Error())
		}
		if hash != e.RequestHash {
			return nil, newCorrupt(lineNum, "hash mismatch")
		}

		if e.Response != nil && e.Error != "" {
			return nil, newCorrupt(lineNum, "both response and error present")
		}
		if e.Response == nil && e.Error == "" {
			return nil, newCorrupt(lineNum, "neither response nor error present")
		}

		entries = append(entries, replayEntry{
			seq:          e.Seq,
			line:         lineNum,
			requestHash:  e.RequestHash,
			request:      e.Request,
			response:     e.Response,
			errorMessage: e.Error,
		})

		if len(entries) > 100_000 {
			return nil, newTooLarge(lineNum, "too many entries")
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, newTooLarge(lineNum, "line too long")
		}
		return nil, fmt.Errorf("llmsessionreplay: reading session: %w", err)
	}
	return &Replayer{entries: entries}, nil
}

// Complete returns the recorded response for a matching request, or an error.
// It checks ctx.Err() first and consumes an entry only on match.
func (rp *Replayer) Complete(ctx context.Context, req Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.next >= len(rp.entries) {
		return Response{}, newExhausted()
	}
	ent := rp.entries[rp.next]

	hash, err := computeRequestHash(req)
	if err != nil {
		return Response{}, fmt.Errorf("llmsessionreplay: computing request hash: %w", err)
	}
	if hash != ent.requestHash {
		field := firstMismatchField(ent.request, req)
		return Response{}, newMismatch(ent.seq, field, ent.line)
	}

	rp.next++
	if ent.errorMessage != "" {
		return Response{}, &RecordedError{Message: ent.errorMessage}
	}
	if ent.response == nil {
		return Response{}, newCorrupt(ent.line, "recorded entry missing response")
	}
	return *ent.response, nil
}

// Remaining returns the number of recorded calls not yet replayed.
func (rp *Replayer) Remaining() int {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	return len(rp.entries) - rp.next
}

// Done returns nil if all recorded calls were replayed, otherwise ErrUnconsumed.
func (rp *Replayer) Done() error {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	remaining := len(rp.entries) - rp.next
	if remaining > 0 {
		return newUnconsumed(remaining)
	}
	return nil
}

// computeRequestHash returns the lowercase hex SHA-256 of the canonical JSON encoding of req.
func computeRequestHash(req Request) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(req); err != nil {
		return "", err
	}
	b := buf.Bytes()
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// firstMismatchField returns the name of the first field that differs between a and b,
// or empty string if they are equal.
func firstMismatchField(a, b Request) string {
	if a.Model != b.Model {
		return "model"
	}
	if a.MaxTokens != b.MaxTokens {
		return "max_tokens"
	}
	if (a.Temperature == nil) != (b.Temperature == nil) {
		return "temperature"
	}
	if a.Temperature != nil && b.Temperature != nil && *a.Temperature != *b.Temperature {
		return "temperature"
	}
	if len(a.Messages) != len(b.Messages) {
		return "messages"
	}
	for i := range a.Messages {
		if a.Messages[i].Role != b.Messages[i].Role {
			return fmt.Sprintf("messages[%d].role", i)
		}
		if a.Messages[i].Content != b.Messages[i].Content {
			return fmt.Sprintf("messages[%d].content", i)
		}
	}
	return ""
}
