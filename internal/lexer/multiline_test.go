package lexer

import (
	"strings"
	"testing"

	"abc/internal/token"
)

func multiValue(t *testing.T, src string) string {
	t.Helper()
	res, err := Scan("t.pas", []byte(src))
	if err != nil {
		t.Fatalf("scan(%q): %v", src, err)
	}
	if res.Tokens[0].Kind != token.MultiStr {
		t.Fatalf("scan(%q): first token %s, want multiline string", src, res.Tokens[0].Kind)
	}
	return res.Tokens[0].Value
}

// The expectations below were produced with the original PascalABC.NET
// compiler (tools/oracle.sh).
func TestMultilineStringValue(t *testing.T) {
	tests := []struct{ src, want string }{
		{"'''\nabc\n'''", "abc"},
		{"'''\nline1\nline2\n'''", "line1\nline2"},
		{"'''  \nabc\n'''", "abc"},
		// the common indentation of the content lines is stripped
		{"'''\n    line1\n    line2\n    '''", "line1\nline2"},
		// ... but only the common part: 0 is the smallest indentation here
		{"'''\na\n  b\n'''", "a\n  b"},
		// whitespace-only lines are content and count for the indentation
		{"'''\n    a\n     \n    '''", "a\n "},
		// a trailing blank line stays (only the last newline is dropped)
		{"'''\na\n\n'''", "a\n"},
		{"'''\na\n\nb\n'''", "a\n\nb"},
		// quotes are not escapes inside a multiline string
		{"'''\nit''s a 'b' c\n'''", "it''s a 'b' c"},
		{"'''\n'''", ""},
	}
	for _, tc := range tests {
		if got := multiValue(t, tc.src); got != tc.want {
			t.Errorf("scan(%q) = %q, want %q", tc.src, got, tc.want)
		}
	}
}

func TestMultilineStringErrors(t *testing.T) {
	tests := []struct{ src, want string }{
		// the closing quotes have to start the line
		{"'''\nabc   '''", "There should be no non-whitespace characters before the closing quotes of the multiline string"},
		{"'''\nabc\n   '''", "Multiline string contains inconsistent indents"},
		{"'''\n    a\n  b\n    '''", "Multiline string contains inconsistent indents"},
		// a blank line is counted as indentation 0
		{"'''\n    a\n\n    '''", "Multiline string contains inconsistent indents"},
		{"'''\nabc\n", "unterminated multiline string literal"},
	}
	for _, tc := range tests {
		_, err := Scan("t.pas", []byte(tc.src))
		if err == nil {
			t.Errorf("scan(%q): no error, want %q", tc.src, tc.want)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("scan(%q): error %q, want it to contain %q", tc.src, err, tc.want)
		}
	}
}
