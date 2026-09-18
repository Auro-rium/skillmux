package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Auro-rium/skillmux/internal/app"
)

func Run(a *app.App) error {
	reader := bufio.NewReader(os.Stdin)
	query := ""
	for {
		skills, err := a.Search(query)
		if err != nil { return err }
		status, _ := a.Status()
		fmt.Print("\x1b[2J\x1b[H")
		fmt.Printf("skillmux                                           %s\n", status.Profile)
		fmt.Println(strings.Repeat("─", 72))
		fmt.Printf("Skills: %-4d  Enabled: %-4d  Conflicts: %-3d  Doctor: %dW/%dE\n",
			status.Skills, status.Enabled, status.Conflicts, status.DoctorWarnings, status.DoctorErrors)
		fmt.Println(strings.Repeat("─", 72))
		if query != "" { fmt.Printf("filter: %s\n", query) }
		for i, sk := range skills {
			if i >= 18 {
				fmt.Printf("… %d more\n", len(skills)-i)
				break
			}
			var targets []string
			for h, enabled := range sk.Enabled { if enabled { targets=append(targets,h) } }
			fmt.Printf("%-24s %-10s %s\n", sk.Name, sk.Health, strings.Join(targets, ","))
		}
		if len(skills)==0 { fmt.Println("No managed skills. Run: skillmux scan  or  skillmux add <source>") }
		fmt.Println(strings.Repeat("─", 72))
		fmt.Print("/ search   a add   s sync plan   d doctor   q quit\n> ")
		line, err := reader.ReadString('\n')
		if err != nil { return nil }
		line=strings.TrimSpace(line)
		switch {
		case line=="q" || line=="quit":
			return nil
		case line=="/":
			fmt.Print("search: ")
			q, _ := reader.ReadString('\n')
			query=strings.TrimSpace(q)
		case strings.HasPrefix(line, "/"):
			query=strings.TrimSpace(strings.TrimPrefix(line, "/"))
		case line=="d":
			report, err := a.Doctor()
			if err != nil { return err }
			fmt.Printf("\nDoctor checked %d skills\n", report.SkillsChecked)
			for _, f := range report.Findings { fmt.Printf("%s %-22s %s\n", strings.ToUpper(f.Severity), f.Skill, f.Message) }
			pause(reader)
		case line=="s":
			plan, err := a.Sync(true, false)
			if err != nil { return err }
			fmt.Printf("\nSync plan: %d change(s)\n", len(plan.Changes))
			for _, c := range plan.Changes { fmt.Printf("%-8s %-12s %s -> %s\n", c.Kind, c.Harness, c.Skill, c.Target) }
			fmt.Println("Use 'skillmux sync' to apply.")
			pause(reader)
		case line=="a":
			fmt.Println("Use: skillmux add <path|github:owner/repo> --target codex,claude")
			pause(reader)
		default:
			if line!="" { fmt.Println("Unknown command."); pause(reader) }
		}
	}
}

func pause(r *bufio.Reader) {
	fmt.Print("\n[Enter] continue ")
	_, _ = r.ReadString('\n')
}
