package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Auro-rium/skillmux/internal/app"
	"github.com/Auro-rium/skillmux/internal/core"
	"github.com/Auro-rium/skillmux/internal/ui"
)

var version = "dev"

func main() {
	a, err := app.Open()
	if err != nil {
		fatal(err)
	}
	if len(os.Args) == 1 {
		if err := ui.Run(a); err != nil {
			fatal(err)
		}
		return
	}

	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "scan":
		fs, jsonOut := commonFlags("scan")
		parse(fs, args)
		v, err := a.Scan()
		must(err)
		if *jsonOut { printJSON(v) } else { renderScan(v) }
	case "status":
		fs, jsonOut := commonFlags("status")
		parse(fs, args)
		v, err := a.Status()
		must(err)
		if *jsonOut { printJSON(v) } else { renderStatus(v) }
	case "list":
		fs, jsonOut := commonFlags("list")
		parse(fs, args)
		v, err := a.List()
		must(err)
		if *jsonOut { printJSON(v) } else { renderList(v) }
	case "get":
		fs, jsonOut := commonFlags("get")
		parse(fs, args)
		require(fs, 1)
		v, err := a.Get(fs.Arg(0))
		must(err)
		if *jsonOut { printJSON(v) } else { renderSkill(v) }
	case "search":
		fs, jsonOut := commonFlags("search")
		parse(fs, args)
		require(fs, 1)
		v, err := a.Search(strings.Join(fs.Args(), " "))
		must(err)
		if *jsonOut { printJSON(v) } else { renderList(v) }
	case "add":
		runAdd(a, args)
	case "import":
		runImport(a, args)
	case "update":
		runUpdate(a, args)
	case "sync":
		runSync(a, args)
	case "diff":
		runDiff(a, args)
	case "doctor":
		fs, jsonOut := commonFlags("doctor")
		parse(fs, args)
		v, err := a.Doctor()
		must(err)
		if *jsonOut { printJSON(v) } else { renderDoctor(v) }
	case "enable":
		runToggle(a, args, true)
	case "disable":
		runToggle(a, args, false)
	case "remove":
		runRemove(a, args)
	case "profile":
		runProfile(a, args)
	case "eject":
		fs := flag.NewFlagSet("eject", flag.ContinueOnError)
		parse(fs, args)
		name := ""
		if fs.NArg() > 0 { name = fs.Arg(0) }
		must(a.Eject(name))
		if name == "" { fmt.Println("Ejected all managed links into regular directories.") } else { fmt.Printf("Ejected %s.\n", name) }
	case "completion":
		runCompletion(args)
	case "version", "--version", "-v":
		fmt.Printf("skillmux %s\n", version)
	case "help", "--help", "-h":
		help()
	default:
		fatal(fmt.Errorf("unknown command %q; run 'skillmux help'", cmd))
	}
}

func commonFlags(name string) (*flag.FlagSet, *bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs, fs.Bool("json", false, "emit stable JSON")
}

func parse(fs *flag.FlagSet, args []string) {
	if err := fs.Parse(reorderFlags(args)); err != nil {
		os.Exit(2)
	}
}

func reorderFlags(args []string) []string {
	valueFlags := map[string]bool{"--target": true, "-target": true, "--from": true, "-from": true}
	var opts, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			opts = append(opts, a)
			name := a
			if j := strings.IndexByte(a, '='); j >= 0 { name = a[:j] }
			if valueFlags[name] && !strings.Contains(a, "=") && i+1 < len(args) {
				i++
				opts = append(opts, args[i])
			}
		} else {
			pos = append(pos, a)
		}
	}
	return append(opts, pos...)
}

func require(fs *flag.FlagSet, n int) {
	if fs.NArg() < n {
		fatal(fmt.Errorf("%s requires an argument", fs.Name()))
	}
}

func runAdd(a *app.App, args []string) {
	fs, jsonOut := commonFlags("add")
	targets := fs.String("target", "", "comma-separated target harnesses")
	dry := fs.Bool("dry-run", false, "show intended add without changing state")
	parse(fs, args)
	require(fs, 1)
	if *dry {
		fmt.Printf("Would add %s", fs.Arg(0))
		if *targets != "" { fmt.Printf(" to %s", *targets) }
		fmt.Println()
		return
	}
	var targetList []string
	if *targets != "" { targetList = strings.Split(*targets, ",") }
	v, err := a.Add(context.Background(), fs.Arg(0), targetList)
	must(err)
	if *jsonOut { printJSON(v) } else { renderSkill(v) }
}

func runImport(a *app.App, args []string) {
	fs, jsonOut := commonFlags("import")
	from := fs.String("from", "", "choose a conflicting copy from this harness")
	parse(fs, args)
	name := ""
	if fs.NArg() > 0 { name = fs.Arg(0) }
	v, err := a.Import(name, *from)
	must(err)
	if *jsonOut {
		printJSON(v)
		return
	}
	for _, n := range v.Imported { fmt.Printf("+ imported %s\n", n) }
	for _, n := range v.Conflicts { fmt.Printf("! conflict %s: use 'skillmux import %s --from <harness>'\n", n, n) }
	for _, n := range v.Skipped { fmt.Printf("- skipped %s\n", n) }
}

func runUpdate(a *app.App, args []string) {
	fs, jsonOut := commonFlags("update")
	parse(fs, args)
	name := ""
	if fs.NArg() > 0 { name = fs.Arg(0) }
	v, err := a.Update(context.Background(), name)
	must(err)
	if *jsonOut {
		printJSON(v)
		return
	}
	if len(v) == 0 {
		fmt.Println("No updates.")
		return
	}
	for _, n := range v { fmt.Printf("~ updated %s\n", n) }
}

func runSync(a *app.App, args []string) {
	fs, jsonOut := commonFlags("sync")
	dry := fs.Bool("dry-run", false, "print plan only")
	yes := fs.Bool("yes", false, "apply without confirmation")
	force := fs.Bool("force", false, "replace differing targets after review")
	parse(fs, args)

	plan, err := a.ProjectSync(context.Background(), true, *force)
	must(err)
	if *jsonOut { printJSON(plan) } else { renderPlan(plan) }
	if *dry || len(plan.Changes) == 0 { return }

	if !*yes {
		if !app.Confirm(bufio.NewReader(os.Stdin), "Apply? [Y/n] ") {
			fmt.Println("Cancelled.")
			return
		}
	}
	_, err = a.ProjectSync(context.Background(), false, *force)
	must(err)
	if !*jsonOut { fmt.Println("Sync complete.") }
}

func runDiff(a *app.App, args []string) {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	target := fs.String("target", "", "harness to compare against")
	parse(fs, args)
	require(fs, 1)
	v, err := a.Diff(fs.Arg(0), *target)
	must(err)
	fmt.Print(v)
}

func runToggle(a *app.App, args []string, enabled bool) {
	name := "enable"
	if !enabled { name = "disable" }
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	target := fs.String("target", "", "target harness")
	parse(fs, args)
	require(fs, 1)
	if *target == "" {
		fatal(fmt.Errorf("%s requires --target in V1 to avoid accidental global exposure", name))
	}
	must(a.Enable(fs.Arg(0), *target, enabled))
	fmt.Printf("%s %s for %s. Run 'skillmux sync' to apply.\n", strings.ToUpper(name[:1])+name[1:], fs.Arg(0), *target)
}

func runRemove(a *app.App, args []string) {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	target := fs.String("target", "", "remove exposure from one target")
	canonical := fs.Bool("canonical", false, "eject links and remove canonical copy")
	dry := fs.Bool("dry-run", false, "show intended removal")
	parse(fs, args)
	require(fs, 1)
	if *dry {
		if *canonical { fmt.Printf("Would eject and remove canonical %s\n", fs.Arg(0)) } else { fmt.Printf("Would disable %s for %s\n", fs.Arg(0), *target) }
		return
	}
	must(a.Remove(fs.Arg(0), *target, *canonical))
	fmt.Println("Done.")
}

func runProfile(a *app.App, args []string) {
	if len(args) == 0 { fatal(fmt.Errorf("profile requires list, show, create, use, or delete")) }
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		names, err := a.Store.ListProfiles()
		must(err)
		st, _ := a.Store.LoadState()
		for _, n := range names {
			m := " "
			if n == st.ActiveProfile { m = "*" }
			fmt.Printf("%s %s\n", m, n)
		}
	case "show":
		if len(rest) < 1 { fatal(fmt.Errorf("profile show requires a name")) }
		v, err := a.Store.LoadProfile(rest[0])
		must(err)
		printJSON(v)
	case "create":
		if len(rest) < 1 { fatal(fmt.Errorf("profile create requires a name")) }
		v, err := a.ProfileCreate(rest[0])
		must(err)
		fmt.Printf("Created profile %s with %d skill(s).\n", v.Name, len(v.Skills))
	case "use":
		if len(rest) < 1 { fatal(fmt.Errorf("profile use requires a name")) }
		v, err := a.ProfileUse(rest[0])
		must(err)
		fmt.Printf("Using profile %s. Run 'skillmux sync' to apply.\n", v.Name)
	case "delete":
		if len(rest) < 1 { fatal(fmt.Errorf("profile delete requires a name")) }
		must(a.ProfileDelete(rest[0]))
		fmt.Printf("Deleted profile %s.\n", rest[0])
	default:
		fatal(fmt.Errorf("unknown profile command %q", sub))
	}
}

func runCompletion(args []string) {
	if len(args) < 1 { fatal(fmt.Errorf("completion requires bash, zsh, fish, or powershell")) }
	commands := "scan status list get search add import remove update sync diff doctor enable disable profile eject completion version"
	switch strings.ToLower(args[0]) {
	case "bash":
		fmt.Printf("complete -W %q skillmux\n", commands)
	case "zsh":
		fmt.Printf("#compdef skillmux\n_arguments '1:command:(%s)'\n", commands)
	case "fish":
		fmt.Printf("complete -c skillmux -f -a %q\n", commands)
	case "powershell":
		fmt.Println("Register-ArgumentCompleter -Native -CommandName skillmux -ScriptBlock { param($wordToComplete) 'scan','status','list','get','search','add','import','remove','update','sync','diff','doctor','enable','disable','profile','eject','completion','version' | Where-Object { $_ -like \"$wordToComplete*\" } }")
	default:
		fatal(fmt.Errorf("unsupported shell %q", args[0]))
	}
}

func printJSON(v any) {
	b, err := app.EncodeJSON(v)
	must(err)
	fmt.Print(string(b))
}

func renderScan(r core.ScanReport) {
	fmt.Println("Harnesses")
	for _, h := range r.Harnesses {
		m := "○"
		if h.Detected { m = "✓" }
		fmt.Printf("%s %s\n", m, h.Name)
	}
	fmt.Printf("\nSkills found: %d\nUnique skills: %d\nDuplicates: %d\nConflicts: %d\nBroken: %d\n", len(r.Installations), r.UniqueSkills, r.Duplicates, len(r.Conflicts), r.Broken)
	for _, c := range r.Conflicts {
		fmt.Printf("\n! %s: %s\n", c.Name, c.Reason)
		for _, cp := range c.Copies { fmt.Printf("  %-10s %s\n", cp.Harness, cp.Path) }
	}
}

func renderStatus(s app.Status) {
	fmt.Printf("Profile: %s\n\nHarnesses\n", s.Profile)
	for _, h := range s.Harnesses {
		m := "○"
		if h.Detected { m = "✓" }
		fmt.Printf("%s %s\n", m, h.Name)
	}
	fmt.Printf("\nSkills: %d\nEnabled: %d\nPending sync changes: %d\nConflicts: %d\nDoctor: %d warnings, %d errors\n", s.Skills, s.Enabled, s.PendingChanges, s.Conflicts, s.DoctorWarnings, s.DoctorErrors)
}

func renderList(skills []core.Skill) {
	fmt.Printf("%-24s %-24s %-10s %s\n", "NAME", "SOURCE", "HEALTH", "TARGETS")
	for _, sk := range skills {
		src := "local"
		if sk.Source != nil {
			if sk.Source.Repository != "" { src = sk.Source.Repository } else if sk.Source.URL != "" { src = sk.Source.URL }
		}
		var targets []string
		for h, enabled := range sk.Enabled { if enabled { targets = append(targets, h) } }
		sort.Strings(targets)
		fmt.Printf("%-24s %-24s %-10s %s\n", sk.Name, truncate(src, 24), sk.Health, strings.Join(targets, ","))
	}
}

func renderSkill(sk core.Skill) {
	fmt.Printf("%s\n", sk.Name)
	if sk.Description != "" { fmt.Println(sk.Description) }
	fmt.Printf("\nPath: %s\nHash: %s\nHealth: %s\nModified: %t\n", sk.Path, sk.Hash, sk.Health, sk.Modified)
	if sk.Source != nil { fmt.Printf("Source: %s\nCommit: %s\n", sk.Source.URL, sk.Source.Commit) }
	var targets []string
	for h, enabled := range sk.Enabled { if enabled { targets = append(targets, h) } }
	sort.Strings(targets)
	fmt.Printf("Enabled: %s\n", strings.Join(targets, ", "))
}

func renderDoctor(r core.DoctorReport) {
	fmt.Printf("Doctor\n\n%d skills checked\n", r.SkillsChecked)
	warnings, errs := 0, 0
	for _, f := range r.Findings {
		if f.Severity == "error" { errs++ } else { warnings++ }
		fmt.Printf("%-7s %-22s %s\n", strings.ToUpper(f.Severity), f.Skill, f.Message)
	}
	if len(r.Findings) == 0 { fmt.Println("✓ no findings") }
	fmt.Printf("\n%d warnings\n%d errors\n", warnings, errs)
}

func renderPlan(p core.Plan) {
	fmt.Println("Sync plan")
	if len(p.Changes) == 0 {
		fmt.Println("No changes.")
		return
	}
	current := ""
	for _, c := range p.Changes {
		if c.Harness != current {
			current = c.Harness
			fmt.Printf("\n%s\n", current)
		}
		symbol := map[core.ChangeKind]string{core.ChangeInstall: "+", core.ChangeUpdate: "~", core.ChangeRemove: "-", core.ChangeRepair: "!"}[c.Kind]
		fmt.Printf("  %s %s", symbol, c.Skill)
		if c.Reason != "" { fmt.Printf("  (%s)", c.Reason) }
		fmt.Println()
	}
	fmt.Printf("\n%d change(s)\n", len(p.Changes))
}

func truncate(s string, n int) string {
	if len(s) <= n { return s }
	if n < 2 { return s[:n] }
	return s[:n-1] + "…"
}

func must(err error) {
	if err != nil { fatal(err) }
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "skillmux: %v\n", err)
	os.Exit(1)
}

func help() {
	fmt.Print(`Skillmux — One skill library. Every agent.

Usage:
  skillmux                         launch the local TUI
  skillmux scan [--json]           discover harnesses, skills, conflicts
  skillmux status [--json]         summarize local state
  skillmux list [--json]           list canonical skills
  skillmux get NAME [--json]       inspect one skill
  skillmux search QUERY [--json]   search managed skills
  skillmux add SOURCE              add local/GitHub skill
  skillmux import [NAME]           adopt unmanaged skills
  skillmux update [NAME]           update GitHub-backed skills
  skillmux sync [--dry-run]        plan/apply harness exposure
  skillmux diff NAME               compare canonical and harness copy
  skillmux doctor [--json]         static diagnostics
  skillmux enable NAME --target H  enable for one harness
  skillmux disable NAME --target H disable for one harness
  skillmux remove NAME             conservative removal
  skillmux profile ...             manage profiles
  skillmux eject [NAME]            replace managed links with regular files
  skillmux completion SHELL        print shell completion
  skillmux version                 print version
`)
}
