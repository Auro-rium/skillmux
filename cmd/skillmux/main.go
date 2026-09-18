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
	if err != nil { fatal(err) }
	if len(os.Args)==1 {
		if err := ui.Run(a); err != nil { fatal(err) }
		return
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "scan":
		fs, jsonOut := flags("scan")
		mustParse(fs,args)
		v, err := a.Scan()
		output(v,*jsonOut,renderScan)
		check(err)
	case "status":
		fs, jsonOut := flags("status")
		mustParse(fs,args)
		v, err := a.Status()
		output(v,*jsonOut,renderStatus)
		check(err)
	case "list":
		fs, jsonOut := flags("list")
		mustParse(fs,args)
		v, err := a.List()
		output(v,*jsonOut,renderList)
		check(err)
	case "get":
		fs, jsonOut := flags("get")
		mustParse(fs,args)
		requireArgs(fs,1)
		v, err := a.Get(fs.Arg(0))
		output(v,*jsonOut,renderSkill)
		check(err)
	case "search":
		fs, jsonOut := flags("search")
		mustParse(fs,args)
		requireArgs(fs,1)
		v, err := a.Search(strings.Join(fs.Args()," "))
		output(v,*jsonOut,renderList)
		check(err)
	case "add":
		addCmd(a,args)
	case "import":
		importCmd(a,args)
	case "sync":
		syncCmd(a,args)
	case "doctor":
		fs, jsonOut := flags("doctor")
		mustParse(fs,args)
		v, err := a.Doctor()
		output(v,*jsonOut,renderDoctor)
		check(err)
	case "diff":
		diffCmd(a,args)
	case "enable":
		enableCmd(a,args,true)
	case "disable":
		enableCmd(a,args,false)
	case "remove":
		removeCmd(a,args)
	case "eject":
		fs := flag.NewFlagSet("eject",flag.ContinueOnError)
		mustParse(fs,args)
		name := ""
		if fs.NArg()>0 { name=fs.Arg(0) }
		check(a.Eject(name))
		if name=="" { fmt.Println("Ejected all managed links into regular skill directories.") } else { fmt.Printf("Ejected %s.\n",name) }
	case "update":
		updateCmd(a,args)
	case "profile":
		profileCmd(a,args)
	case "completion":
		completionCmd(args)
	case "version", "--version", "-v":
		fmt.Printf("skillmux %s\n",version)
	case "help", "--help", "-h":
		help()
	default:
		fatal(fmt.Errorf("unknown command %q; run 'skillmux help'",cmd))
	}
}

func flags(name string) (*flag.FlagSet,*bool) {
	fs:=flag.NewFlagSet(name,flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut:=fs.Bool("json",false,"emit stable JSON")
	return fs,jsonOut
}

func mustParse(fs *flag.FlagSet,args []string) {
	if err:=fs.Parse(args); err!=nil { os.Exit(2) }
}

func requireArgs(fs *flag.FlagSet,n int) {
	if fs.NArg()<n { fatal(fmt.Errorf("%s requires an argument",fs.Name())) }
}

func addCmd(a *app.App,args []string) {
	fs,jsonOut:=flags("add")
	targets:=fs.String("target","","comma-separated targets: codex,claude,gemini,cursor,opencode")
	dry:=fs.Bool("dry-run",false,"inspect source without installing")
	mustParse(fs,args)
	requireArgs(fs,1)
	raw:=fs.Arg(0)
	if *dry {
		fmt.Printf("Would add %s",raw)
		if *targets!="" { fmt.Printf(" to %s",*targets) }
		fmt.Println()
		return
	}
	var t []string
	if *targets!="" { t=strings.Split(*targets,",") }
	v,err:=a.Add(context.Background(),raw,t)
	output(v,*jsonOut,renderSkill)
	check(err)
}

func importCmd(a *app.App,args []string) {
	fs,jsonOut:=flags("import")
	from:=fs.String("from","","choose a conflicting copy from this harness")
	mustParse(fs,args)
	name:=""
	if fs.NArg()>0 { name=fs.Arg(0) }
	v,err:=a.Import(name,*from)
	output(v,*jsonOut,func(v any){
		r:=v.(app.ImportResult)
		for _,n:=range r.Imported { fmt.Printf("+ imported %s\n",n) }
		for _,n:=range r.Conflicts { fmt.Printf("! conflict %s: rerun 'skillmux import %s --from <harness>'\n",n,n) }
		for _,n:=range r.Skipped { fmt.Printf("- skipped %s\n",n) }
	})
	check(err)
}

func syncCmd(a *app.App,args []string) {
	fs,jsonOut:=flags("sync")
	dry:=fs.Bool("dry-run",false,"print the deterministic plan only")
	yes:=fs.Bool("yes",false,"apply without confirmation")
	force:=fs.Bool("force",false,"replace differing targets after reviewing the plan/diff")
	mustParse(fs,args)
	plan,err:=a.Sync(true,*force)
	if err!=nil { fatal(err) }
	if *jsonOut {
		output(plan,true,nil)
		if *dry { return }
	} else {
		renderPlan(plan)
		if *dry || len(plan.Changes)==0 { return }
	}
	if !*yes {
		reader:=bufio.NewReader(os.Stdin)
		if !app.Confirm(reader,"Apply? [Y/n] ") { fmt.Println("Cancelled."); return }
	}
	_,err=a.Sync(false,*force)
	check(err)
	if !*jsonOut { fmt.Println("Sync complete.") }
}

func diffCmd(a *app.App,args []string) {
	fs:=flag.NewFlagSet("diff",flag.ContinueOnError)
	target:=fs.String("target","","harness to compare against")
	mustParse(fs,args)
	requireArgs(fs,1)
	v,err:=a.Diff(fs.Arg(0),*target)
	check(err)
	fmt.Print(v)
}

func enableCmd(a *app.App,args []string,enabled bool) {
	name:="enable"; if !enabled { name="disable" }
	fs:=flag.NewFlagSet(name,flag.ContinueOnError)
	target:=fs.String("target","","target harness")
	mustParse(fs,args)
	requireArgs(fs,1)
	if *target=="" { fatal(errorsText(name+" requires --target in V1 to avoid accidental global exposure")) }
	check(a.Enable(fs.Arg(0),*target,enabled))
	fmt.Printf("%s %s for %s. Run 'skillmux sync' to apply.\n",strings.Title(name),fs.Arg(0),*target)
}

func removeCmd(a *app.App,args []string) {
	fs:=flag.NewFlagSet("remove",flag.ContinueOnError)
	target:=fs.String("target","","remove exposure from one target")
	canonical:=fs.Bool("canonical",false,"eject managed links and remove canonical Skillmux copy")
	dry:=fs.Bool("dry-run",false,"show intended operation")
	mustParse(fs,args)
	requireArgs(fs,1)
	if *dry {
		if *canonical { fmt.Printf("Would eject and remove canonical %s\n",fs.Arg(0)) } else { fmt.Printf("Would disable %s for %s\n",fs.Arg(0),*target) }
		return
	}
	check(a.Remove(fs.Arg(0),*target,*canonical))
	fmt.Println("Done.")
}

func updateCmd(a *app.App,args []string) {
	fs,jsonOut:=flags("update")
	mustParse(fs,args)
	only:=""
	if fs.NArg()>0 { only=fs.Arg(0) }
	v,err:=a.Update(context.Background(),only)
	output(v,*jsonOut,func(v any){
		names:=v.([]string)
		if len(names)==0 { fmt.Println("No updates.") ; return }
		for _,n:=range names { fmt.Printf("~ updated %s\n",n) }
	})
	check(err)
}

func profileCmd(a *app.App,args []string) {
	if len(args)==0 { fatal(errorsText("profile requires: list, show, create, use, delete")) }
	sub:=args[0]
	rest:=args[1:]
	switch sub {
	case "list":
		v,err:=a.Store.ListProfiles(); check(err)
		st,_:=a.Store.LoadState()
		for _,n:=range v {
			marker:=" "; if n==st.ActiveProfile { marker="*" }
			fmt.Printf("%s %s\n",marker,n)
		}
	case "show":
		if len(rest)<1 { fatal(errorsText("profile show requires a name")) }
		v,err:=a.Store.LoadProfile(rest[0]); check(err)
		b,_:=app.EncodeJSON(v); fmt.Print(string(b))
	case "create":
		if len(rest)<1 { fatal(errorsText("profile create requires a name")) }
		v,err:=a.ProfileCreate(rest[0]); check(err)
		fmt.Printf("Created profile %s with %d skill(s).\n",v.Name,len(v.Skills))
	case "use":
		if len(rest)<1 { fatal(errorsText("profile use requires a name")) }
		v,err:=a.ProfileUse(rest[0]); check(err)
		fmt.Printf("Using profile %s. Run 'skillmux sync' to apply.\n",v.Name)
	case "delete":
		if len(rest)<1 { fatal(errorsText("profile delete requires a name")) }
		check(a.ProfileDelete(rest[0])); fmt.Printf("Deleted profile %s.\n",rest[0])
	default:
		fatal(fmt.Errorf("unknown profile command %q",sub))
	}
}

func completionCmd(args []string) {
	if len(args)<1 { fatal(errorsText("completion requires bash, zsh, fish, or powershell")) }
	switch strings.ToLower(args[0]) {
	case "bash":
		fmt.Println("complete -W \"scan status list get search add import remove update sync diff doctor enable disable profile eject completion version\" skillmux")
	case "zsh":
		fmt.Println("#compdef skillmux\n_arguments '1:command:(scan status list get search add import remove update sync diff doctor enable disable profile eject completion version)'")
	case "fish":
		fmt.Println("complete -c skillmux -f -a 'scan status list get search add import remove update sync diff doctor enable disable profile eject completion version'")
	case "powershell":
		fmt.Println("Register-ArgumentCompleter -Native -CommandName skillmux -ScriptBlock { param($wordToComplete) 'scan','status','list','get','search','add','import','remove','update','sync','diff','doctor','enable','disable','profile','eject','completion','version' | Where-Object { $_ -like \"$wordToComplete*\" } }")
	default:
		fatal(errorsText("unsupported shell"))
	}
}

func output(v any,jsonOut bool,render func(any)) {
	if jsonOut {
		b,err:=app.EncodeJSON(v); check(err); fmt.Print(string(b)); return
	}
	if render!=nil { render(v) }
}

func renderScan(v any) {
	r:=v.(core.ScanReport)
	fmt.Println("Harnesses")
	for _,h:=range r.Harnesses {
		marker:="○"; if h.Detected { marker="✓" }
		fmt.Printf("%s %s\n",marker,h.Name)
	}
	fmt.Printf("\nSkills found: %d\nUnique skills: %d\nDuplicates: %d\nConflicts: %d\nBroken: %d\n",
		len(r.Installations),r.UniqueSkills,r.Duplicates,len(r.Conflicts),r.Broken)
	for _,c:=range r.Conflicts {
		fmt.Printf("\n! %s: %s\n",c.Name,c.Reason)
		for _,cp:=range c.Copies { fmt.Printf("  %-10s %s\n",cp.Harness,cp.Path) }
	}
}

func renderStatus(v any) {
	s:=v.(app.Status)
	fmt.Printf("Profile: %s\n\nHarnesses\n",s.Profile)
	for _,h:=range s.Harnesses {
		marker:="○"; if h.Detected { marker="✓" }
		fmt.Printf("%s %s\n",marker,h.Name)
	}
	fmt.Printf("\nSkills: %d\nEnabled: %d\nPending sync changes: %d\nConflicts: %d\nDoctor: %d warnings, %d errors\n",
		s.Skills,s.Enabled,s.PendingChanges,s.Conflicts,s.DoctorWarnings,s.DoctorErrors)
}

func renderList(v any) {
	skills:=v.([]core.Skill)
	fmt.Printf("%-24s %-24s %-10s %s\n","NAME","SOURCE","HEALTH","TARGETS")
	for _,sk:=range skills {
		source:="local"
		if sk.Source!=nil {
			if sk.Source.Repository!="" { source=sk.Source.Repository } else if sk.Source.URL!="" { source=sk.Source.URL }
		}
		var targets []string
		for h,e:=range sk.Enabled { if e { targets=append(targets,h) } }
		sort.Strings(targets)
		fmt.Printf("%-24s %-24s %-10s %s\n",sk.Name,trim(source,24),sk.Health,strings.Join(targets,","))
	}
}

func renderSkill(v any) {
	sk:=v.(core.Skill)
	fmt.Printf("%s\n",sk.Name)
	if sk.Description!="" { fmt.Printf("%s\n",sk.Description) }
	fmt.Printf("\nPath: %s\nHash: %s\nHealth: %s\nModified: %t\n",sk.Path,sk.Hash,sk.Health,sk.Modified)
	if sk.Source!=nil { fmt.Printf("Source: %s\nCommit: %s\n",sk.Source.URL,sk.Source.Commit) }
	var targets []string
	for h,e:=range sk.Enabled { if e { targets=append(targets,h) } }
	sort.Strings(targets)
	fmt.Printf("Enabled: %s\n",strings.Join(targets,", "))
}

func renderDoctor(v any) {
	r:=v.(core.DoctorReport)
	fmt.Printf("Doctor\n\n%d skills checked\n",r.SkillsChecked)
	warnings,errs:=0,0
	for _,f:=range r.Findings {
		if f.Severity=="error" { errs++ } else { warnings++ }
		fmt.Printf("%-7s %-22s %s\n",strings.ToUpper(f.Severity),f.Skill,f.Message)
	}
	if len(r.Findings)==0 { fmt.Println("✓ no findings") }
	fmt.Printf("\n%d warnings\n%d errors\n",warnings,errs)
}

func renderPlan(v any) { renderPlanTyped(v.(core.Plan)) }
func renderPlanTyped(p core.Plan) {
	fmt.Println("Sync plan")
	if len(p.Changes)==0 { fmt.Println("No changes."); return }
	current:=""
	for _,c:=range p.Changes {
		if c.Harness!=current { current=c.Harness; fmt.Printf("\n%s\n",current) }
		symbol:=map[core.ChangeKind]string{core.ChangeInstall:"+",core.ChangeUpdate:"~",core.ChangeRemove:"-",core.ChangeRepair:"!"}[c.Kind]
		fmt.Printf("  %s %s",symbol,c.Skill)
		if c.Reason!="" { fmt.Printf("  (%s)",c.Reason) }
		fmt.Println()
	}
	fmt.Printf("\n%d change(s)\n",len(p.Changes))
}

func renderPlan(p core.Plan) { renderPlanTyped(p) }

func trim(s string,n int) string {
	if len(s)<=n { return s }
	if n<2 { return s[:n] }
	return s[:n-1]+"…"
}

func errorsText(s string) error { return fmt.Errorf("%s",s) }

func check(err error) { if err!=nil { fatal(err) } }

func fatal(err error) {
	fmt.Fprintf(os.Stderr,"skillmux: %v\n",err)
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
  skillmux import [NAME]           adopt existing unmanaged skills
  skillmux update [NAME]           update GitHub-backed skills
  skillmux sync [--dry-run]        plan/apply exposure to harnesses
  skillmux diff NAME               compare canonical vs harness copy
  skillmux doctor [--json]         static health/safety diagnostics
  skillmux enable NAME --target H  enable a skill for one harness
  skillmux disable NAME --target H disable a skill for one harness
  skillmux remove NAME             conservative removal
  skillmux profile ...             manage skill profiles
  skillmux eject [NAME]            replace managed links with regular files
  skillmux completion SHELL        print shell completion
  skillmux version                 print version
`)
}
