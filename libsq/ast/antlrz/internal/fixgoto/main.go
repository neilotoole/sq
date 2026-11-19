// Command fixgoto rewrites an ANTLR-generated Go parser so that go vet doesn't
// report unreachable code. ANTLR's Go target ends every rule function with a
// "goto errorExit" after the return, only to keep the errorExit label used.
// fixgoto moves that goto before the return, inside "if false", as proposed
// upstream in https://github.com/antlr/antlr4/pull/4445. The parsers'
// generate.sh scripts run it on the file ANTLR writes, for example:
//
//	go run ./libsq/ast/antlrz/internal/fixgoto libsq/ast/internal/slq/slq_parser.go
//
// It fails unless every errorExit label has exactly one goto to rewrite, so an
// ANTLR upgrade that changes the generated code fails generation.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
)

const trick = "\tgoto errorExit // Trick to prevent compiler error if the label is not used\n"

var (
	label     = []byte("\nerrorExit:\n")
	generated = []byte("\treturn localctx\n" + trick)
	rewritten = []byte("\tif false {\n\t" + trick + "\t}\n\treturn localctx\n")
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: fixgoto PARSER_FILE")
		os.Exit(2)
	}

	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "fixgoto:", err)
		os.Exit(1)
	}
}

func run(path string) error {
	src, err := os.ReadFile(path) //nolint:gosec // G703: path from generate.sh in dev-only tool
	if err != nil {
		return err
	}

	out, err := rewrite(src)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	return os.WriteFile(path, out, 0o600) //nolint:gosec // G703: path from generate.sh in dev-only tool
}

// rewrite moves the goto before the return in every rule function in src.
func rewrite(src []byte) ([]byte, error) {
	labels := bytes.Count(src, label)
	if labels == 0 {
		return nil, errors.New("no errorExit labels")
	}

	if n := bytes.Count(src, generated); n != labels {
		return nil, fmt.Errorf("%d errorExit labels but %d gotos after a return", labels, n)
	}

	return bytes.ReplaceAll(src, generated, rewritten), nil
}
