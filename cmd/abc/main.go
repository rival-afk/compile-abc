// Command abc is a compiler and toolchain for the PascalABC language.
//
// It mirrors the shape of the Go toolchain:
//
//	abc build main.pas     # compile to a native executable
//	abc run main.pas       # build and run
//	abc fmt file.pas       # format sources
//	abc test               # run tests
//	abc version            # print version
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"abc/internal/driver"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "build":
		err = cmdBuild(args)
	case "run":
		err = cmdRun(args)
	case "fmt":
		err = cmdFmt(args)
	case "version":
		fmt.Printf("abc %s (pascalabc compiler)\n", version)
	case "env":
		err = cmdEnv()
	case "parse":
		err = cmdParse(args)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "abc: unknown command %q\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `abc - a compiler for the PascalABC language

Usage:
  abc build [-o out] file.pas   compile a program
  abc run [flags] file.pas      compile and run a program
  abc fmt file.pas ...          format source files
  abc test                      run package tests
  abc env                       print environment information
  abc version                   print version
  abc parse file.pas            print the syntax tree (debugging aid)

`)
}

type buildFlags struct {
	out string
}

func parseBuildFlags(args []string) (buildFlags, []string, error) {
	f := buildFlags{}
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o":
			if i+1 >= len(args) {
				return f, nil, fmt.Errorf("flag -o needs an argument")
			}
			i++
			f.out = args[i]
		case len(a) > 2 && a[:2] == "-o":
			f.out = a[2:]
		default:
			rest = append(rest, a)
		}
	}
	return f, rest, nil
}

func cmdBuild(args []string) error {
	f, rest, err := parseBuildFlags(args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: abc build [-o out] file.pas")
	}
	out := f.out
	if out == "" {
		base := filepath.Base(rest[0])
		out = base[:len(base)-len(filepath.Ext(base))]
	}
	return driver.Build(&driver.Options{Source: rest[0], Output: out})
}

func cmdRun(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: abc run file.pas [program args]")
	}
	src := args[0]
	res, err := driver.Run(&driver.Options{Source: src}, args[1:])
	if err != nil {
		return err
	}
	if res != 0 {
		os.Exit(res)
	}
	return nil
}

func cmdFmt(args []string) error {
	return driver.Fmt(args)
}

func cmdEnv() error {
	return driver.Env()
}

func cmdParse(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: abc parse file.pas")
	}
	return driver.ParseOnly(args[0])
}
