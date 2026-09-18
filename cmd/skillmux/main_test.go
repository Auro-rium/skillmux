package main

import (
	"flag"
	"os"
	"testing"
)

func TestReorderFlagsKeepsValueFlagsWithValues(t *testing.T) {
	args := reorderFlags([]string{"./skill", "--scope", "project", "--target", "codex,claude"})
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	scope := fs.String("scope", "global", "")
	target := fs.String("target", "", "")
	if err := fs.Parse(args); err != nil { t.Fatal(err) }
	if *scope != "project" { t.Fatalf("scope=%q", *scope) }
	if *target != "codex,claude" { t.Fatalf("target=%q", *target) }
	if fs.NArg() != 1 || fs.Arg(0) != "./skill" {
		t.Fatalf("positional args=%v", fs.Args())
	}
}

func TestInteractiveTerminalRejectsPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil { t.Fatal(err) }
	defer r.Close()
	defer w.Close()
	if interactiveTerminal(w) {
		t.Fatal("pipe must not be treated as an interactive terminal")
	}
}

func TestCLIStatusSymbolsASCII(t *testing.T) {
	t.Setenv("SKILLMUX_ASCII", "1")
	ok, off := cliStatusSymbols()
	if ok != "[OK]" || off != "[OFF]" {
		t.Fatalf("unexpected ASCII markers: %q %q", ok, off)
	}
}
