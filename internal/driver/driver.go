// Package driver implements the abc build pipeline.
package driver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"abc/internal/ast"
	"abc/internal/lexer"
	"abc/internal/parser"
)

// Options control a compilation.
type Options struct {
	Source string
	Output string
	Work   string // intermediate directory (default: .abc)
}

// ParseOnly parses a file and prints its syntax tree.
func ParseOnly(path string) error {
	prog, err := parseFile(path)
	if err != nil {
		return err
	}
	var b strings.Builder
	dumpNode(&b, prog, 0)
	fmt.Print(b.String())
	return nil
}

func parseFile(path string) (*ast.Program, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	res, err := lexer.Scan(path, decodeSource(src))
	if err != nil {
		return nil, err
	}
	prog, err := parser.Parse(res)
	if err != nil {
		return nil, err
	}
	return prog, nil
}

// Build compiles a program to a native executable.
func Build(opts *Options) error {
	return fmt.Errorf("abc build: code generation is not implemented yet")
}

// Run compiles and runs a program, returning its exit code.
func Run(opts *Options, args []string) (int, error) {
	return 1, fmt.Errorf("abc run: code generation is not implemented yet")
}

// Fmt formats PascalABC source files in place.
func Fmt(args []string) error {
	return fmt.Errorf("abc fmt: not implemented yet")
}

// Env prints toolchain information.
func Env() error {
	fmt.Printf("ABCVERSION=%s\n", version())
	if gcc, err := lookPath("gcc"); err == nil {
		fmt.Printf("CC=%s\n", gcc)
	} else {
		fmt.Println("CC=")
	}
	cache := defaultCacheDir()
	fmt.Printf("ABCCACHE=%s\n", cache)
	return nil
}

func version() string { return "0.1.0" }

func lookPath(name string) (string, error) {
	paths := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	for _, dir := range paths {
		cand := filepath.Join(dir, name)
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
	}
	return "", fmt.Errorf("%s not found in PATH", name)
}

func defaultCacheDir() string {
	if d := os.Getenv("ABC_CACHE"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".abc"
	}
	return filepath.Join(home, ".cache", "abc")
}
