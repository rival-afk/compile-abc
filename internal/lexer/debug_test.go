package lexer

import (
	"os"
	"testing"
)

func TestDebugFile(t *testing.T) {
	path := os.Getenv("ABC_DEBUG_FILE")
	if path == "" {
		t.Skip("no ABC_DEBUG_FILE")
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Scan("x", src)
	for i, tok := range res.Tokens {
		t.Logf("%3d %-20s %q at %v", i, tok.Kind, tok.Text, tok.Start)
		if i > 60 {
			break
		}
	}
	if err != nil {
		t.Logf("ERR: %v", err)
	}
}
