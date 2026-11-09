// Command llmsessionreplay verifies the integrity of a session file.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	llmsessionreplay "github.com/Trysac/u209-llm-session-replay"
)

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}

// Run executes the llmsessionreplay command and returns an exit code.
// Subcommands:
//
//	verify <file>  validate a JSONL session file
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: llmsessionreplay verify <file>")
		return 2
	}
	if args[0] != "verify" {
		fmt.Fprintln(stderr, "unknown subcommand")
		return 2
	}
	path := filepath.Clean(args[1])
	dir, name := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	f, err := os.DirFS(dir).Open(name)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer f.Close()
	rp, err := llmsessionreplay.NewReplayer(f)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "ok %d entries\n", rp.Remaining())
	return 0
}
