package main

import (
	"flag"
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
