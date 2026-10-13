package lexer

import (
	"testing"

	"abc/internal/token"
)

func kinds(t *testing.T, src string) []token.Kind {
	t.Helper()
	res, err := Scan("test.pas", []byte(src))
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	var ks []token.Kind
	for _, tok := range res.Tokens {
		ks = append(ks, tok.Kind)
	}
	return ks
}

func expect(t *testing.T, src string, want []token.Kind) {
	t.Helper()
	got := kinds(t, src)
	if len(got) != len(want) {
		t.Fatalf("src=%q got %v want %v", src, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("src=%q token %d: got %s want %s", src, i, got[i], want[i])
		}
	}
}

func TestBasicTokens(t *testing.T) {
	expect(t, "begin writeln('hi') end.",
		[]token.Kind{token.KwBegin, token.Ident, token.LParen, token.Str, token.RParen, token.KwEnd, token.Dot, token.EOF})
}

func TestNumbers(t *testing.T) {
	res, err := Scan("t.pas", []byte("1_000 $FF 1.5 1e10 1.5e-3 1.e5 12bi 1..10"))
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		kind token.Kind
		val  string
	}{
		{token.Int, "1000"},
		{token.Hex, "FF"},
		{token.Float, "1.5"},
		{token.Float, "1e10"},
		{token.Float, "1.5e-3"},
		{token.Float, "1.e5"},
		{token.BigInt, "12bi"},
		{token.Int, "1"},
		{token.DotDot, ""},
		{token.Int, "10"},
		{token.EOF, ""},
	}
	if len(res.Tokens) != len(want) {
		t.Fatalf("got %d tokens: %+v", len(res.Tokens), res.Tokens)
	}
	for i, w := range want {
		got := res.Tokens[i]
		if got.Kind != w.kind {
			t.Errorf("token %d: kind=%s want %s (%q)", i, got.Kind, w.kind, got.Text)
		}
		if w.val != "" && got.Value != w.val {
			t.Errorf("token %d: value=%q want %q", i, got.Value, w.val)
		}
	}
}

func TestStringsAndChars(t *testing.T) {
	res, err := Scan("t.pas", []byte(`'it''s' #65 #$41 $'fmt {0}' 'a'`))
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Kind{token.Str, token.CharLit, token.CharLit, token.FmtStr, token.Str, token.EOF}
	if len(res.Tokens) != len(want) {
		t.Fatalf("tokens: %+v", res.Tokens)
	}
	for i, k := range want {
		if res.Tokens[i].Kind != k {
			t.Errorf("token %d: %s want %s", i, res.Tokens[i].Kind, k)
		}
	}
	if res.Tokens[0].Value != "it's" {
		t.Errorf("string value = %q", res.Tokens[0].Value)
	}
	if res.Tokens[1].Value != "65" {
		t.Errorf("char value = %q", res.Tokens[1].Value)
	}
}

func TestComments(t *testing.T) {
	expect(t, "a // line\n{ block } (* another *) b",
		[]token.Kind{token.Ident, token.Ident, token.EOF})
}

func TestConditionalCompilation(t *testing.T) {
	src := `
{$define DEBUG}
begin
{$ifdef DEBUG}
  writeln('debug');
{$else}
  writeln('release');
{$endif}
{$ifndef DEBUG}
  writeln('nodebug');
{$endif}
end.`
	res, err := Scan("t.pas", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, tok := range res.Tokens {
		if tok.Kind == token.Ident || tok.Kind == token.Str || tok.Kind == token.KwBegin || tok.Kind == token.KwEnd {
			texts = append(texts, tok.Value)
		}
	}
	// DEBUG is defined, so the $ifndef branch is skipped
	want := []string{"begin", "writeln", "debug", "end"}
	if len(texts) != len(want) {
		t.Fatalf("got %v want %v", texts, want)
	}
	for i := range want {
		if texts[i] != want[i] {
			t.Errorf("elem %d: got %q want %q", i, texts[i], want[i])
		}
	}
}

func TestShortProgramMarker(t *testing.T) {
	expect(t, "##\nwriteln(1)", []token.Kind{token.ShortProg, token.Ident, token.LParen, token.Int, token.RParen, token.EOF})
}

func TestCaseInsensitiveKeywords(t *testing.T) {
	expect(t, "BEGIN End WhIlE", []token.Kind{token.KwBegin, token.KwEnd, token.KwWhile, token.EOF})
}

func TestOperators(t *testing.T) {
	expect(t, ":= += -= *= /= ** <> <= >= .. ?: ??",
		[]token.Kind{token.ColonEq, token.PlusEq, token.MinusEq, token.StarEq, token.SlashEq,
			token.StarStar, token.Neq, token.Lte, token.Gte, token.DotDot,
			token.Question, token.Colon, token.DblQuestion, token.EOF})
}

func TestZeroBasedStringsDirective(t *testing.T) {
	res, err := Scan("t.pas", []byte("{$zerobasedstrings}\nbegin end."))
	if err != nil {
		t.Fatal(err)
	}
	if !res.ZeroBasedStrings {
		t.Error("expected ZeroBasedStrings=true")
	}
}

func TestUnicodeIdentifiers(t *testing.T) {
	expect(t, "begin var сумма := 1; end.",
		[]token.Kind{token.KwBegin, token.KwVar, token.Ident, token.ColonEq, token.Int, token.Semicolon, token.KwEnd, token.Dot, token.EOF})
}
