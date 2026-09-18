package doctor

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/Auro-rium/skillmux/internal/core"
	"github.com/Auro-rium/skillmux/internal/scan"
	"github.com/Auro-rium/skillmux/internal/store"
)

var markdownLink = regexp.MustCompile("\\[[^\\]]*\\]\\(([^)]+)\\)")

type Checker struct {
	Store *store.Store
	Scan  *scan.Scanner
}

func New(s *store.Store, sc *scan.Scanner) *Checker { return &Checker{Store:s, Scan:sc} }

func (c *Checker) Run() (core.DoctorReport, error) {
	skills, err := c.Store.ListSkills()
	if err != nil { return core.DoctorReport{}, err }
	report := core.DoctorReport{SkillsChecked:len(skills)}
	for _, sk := range skills {
		path := filepath.Join(sk.Path, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			report.Findings = append(report.Findings, finding("error", sk.Name, path, "missing_skill_md", "SKILL.md is missing or unreadable"))
			continue
		}
		if strings.TrimSpace(string(data)) == "" {
			report.Findings = append(report.Findings, finding("error", sk.Name, path, "empty_skill_md", "SKILL.md is empty"))
		}
		if sk.Description == "" {
			report.Findings = append(report.Findings, finding("warning", sk.Name, path, "missing_description", "skill description could not be determined"))
		}
		for _, m := range markdownLink.FindAllStringSubmatch(string(data), -1) {
			ref := strings.TrimSpace(m[1])
			if ref == "" || strings.Contains(ref, "://") || strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "mailto:") { continue }
			ref = strings.Split(ref, "#")[0]
			if _, err := os.Stat(filepath.Join(sk.Path, filepath.FromSlash(ref))); err != nil {
				report.Findings = append(report.Findings, finding("warning", sk.Name, path, "missing_reference", "referenced file does not exist: "+ref))
			}
		}
		staticChecks(&report, sk, string(data))
		if err := scanScripts(&report, sk); err != nil {
			report.Findings = append(report.Findings, finding("warning", sk.Name, sk.Path, "script_scan_failed", err.Error()))
		}
	}
	scanReport, err := c.Scan.Run()
	if err == nil {
		for _, cf := range scanReport.Conflicts {
			report.Findings = append(report.Findings, finding("warning", cf.Name, "", "diverged_copies", cf.Reason))
		}
		for _, inst := range scanReport.Installations {
			if inst.Broken {
				report.Findings = append(report.Findings, finding("error", inst.SkillName, inst.Path, "broken_installation", "broken or unreadable skill installation"))
			}
		}
	}
	return report, nil
}

func staticChecks(report *core.DoctorReport, sk core.Skill, body string) {
	lower := strings.ToLower(body)
	checks := []struct{needle, code, msg string}{
		{"rm -rf", "destructive_command", "contains a broad recursive delete command; review recommended"},
		{"curl ", "network_command", "contains a curl command; review external downloads before execution"},
		{"wget ", "network_command", "contains a wget command; review external downloads before execution"},
		{"$home", "environment_access", "references HOME/environment state; review portability and secret handling"},
		{"$" + "{home", "environment_access", "references HOME/environment state; review portability and secret handling"},
		{"powershell", "platform_specific", "contains a PowerShell-specific assumption"},
	}
	for _, ch := range checks {
		if strings.Contains(lower, ch.needle) {
			report.Findings = append(report.Findings, finding("warning", sk.Name, sk.Path, ch.code, ch.msg))
		}
	}
	if runtime.GOOS != "windows" && regexp.MustCompile("[A-Za-z]:\\\\").MatchString(body) {
		report.Findings = append(report.Findings, finding("warning", sk.Name, sk.Path, "windows_path", "contains a Windows absolute path"))
	}
}

func scanScripts(report *core.DoctorReport, sk core.Skill) error {
	root := filepath.Join(sk.Path, "scripts")
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if os.IsNotExist(err) { return filepath.SkipDir }
		if err != nil { return err }
		if d.IsDir() { return nil }
		f, err := os.Open(path)
		if err != nil { return err }
		defer f.Close()
		var b strings.Builder
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			b.WriteString(sc.Text())
			b.WriteByte('\n')
		}
		if err := sc.Err(); err != nil { return err }
		staticChecks(report, sk, b.String())
		info, err := d.Info()
		if err == nil && strings.HasSuffix(path, ".sh") && info.Mode().Perm()&0o111 == 0 {
			report.Findings = append(report.Findings, finding("warning", sk.Name, path, "not_executable", "shell script is not executable"))
		}
		return nil
	})
}

func finding(sev, skill, path, code, msg string) core.DoctorFinding {
	return core.DoctorFinding{Severity:sev, Skill:skill, Path:path, Code:code, Message:msg}
}
