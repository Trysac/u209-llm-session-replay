# Demo

This demo shows recording two LLM calls and replaying them, including a mismatch
that leaves the recorded call unconsumed.

```go
package demo

import (
	"bytes"
	"context"
	"fmt"

	llmsessionreplay "github.com/Trysac/u209-llm-session-replay"
)

type mockLLM struct{}

func (mockLLM) Complete(ctx context.Context, req llmsessionreplay.Request) (llmsessionreplay.Response, error) {
	return llmsessionreplay.Response{Text: "response for " + req.Messages[0].Content}, nil
}

func Example() {
	ctx := context.Background()
	var buf bytes.Buffer
	rec := llmsessionreplay.NewRecorder(mockLLM{}, &buf)

	req1 := llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "first"}}}
	req2 := llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "second"}}}

	_, _ = rec.Complete(ctx, req1)
	_, _ = rec.Complete(ctx, req2)

	rp, _ := llmsessionreplay.NewReplayer(&buf)

	resp1, _ := rp.Complete(ctx, req1)
	fmt.Println(resp1.Text)

	// A request that differs from the next recorded one fails with the exact field.
	changed := llmsessionreplay.Request{Model: "m", Messages: []llmsessionreplay.Message{{Role: "user", Content: "changed"}}}
	_, err := rp.Complete(ctx, changed)
	fmt.Println(err)

	// The mismatch did not consume the entry, so req2 still replays.
	fmt.Println(rp.Remaining())
	resp2, _ := rp.Complete(ctx, req2)
	fmt.Println(resp2.Text)
	fmt.Println(rp.Remaining())

	// Output:
	// response for first
	// llmsessionreplay: mismatch at seq 2 field messages[0].content (line 2)
	// 1
	// response for second
	// 0
}
```
