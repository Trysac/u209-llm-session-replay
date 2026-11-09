package llmsessionreplay_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Trysac/u209-llm-session-replay"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "trims and lower-cases", input: "  Hello World ", want: "hello world"},
		{name: "already normal", input: "abc", want: "abc"},
		{name: "keeps inner spacing", input: "A  B", want: "a  b"},
		{name: "tabs and newlines are whitespace", input: "\tX\n", want: "x"},
		{name: "empty is rejected", input: "", wantErr: llmsessionreplay.ErrBlank},
		{name: "whitespace only is rejected", input: " \t\n ", wantErr: llmsessionreplay.ErrBlank},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := llmsessionreplay.Normalize(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Normalize(%q) error = %v, want %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestErrBlankMessage(t *testing.T) {
	if got, want := llmsessionreplay.ErrBlank.Error(), "llmsessionreplay: input is blank"; got != want {
		t.Fatalf("ErrBlank.Error() = %q, want %q", got, want)
	}
}

func ExampleNormalize() {
	out, _ := llmsessionreplay.Normalize("  Hello World ")
	fmt.Println(out)

	_, err := llmsessionreplay.Normalize("   ")
	fmt.Println("rejected:", err)
	// Output:
	// hello world
	// rejected: llmsessionreplay: input is blank
}
