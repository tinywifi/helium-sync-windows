package heliumsync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/huh/v2"
)

func (a App) Restore(allowRunning bool) int {
	if !allowRunning && HeliumRunning() {
		fmt.Println("ERROR: Helium is running on this device.")
		fmt.Println("Restore writes to your Helium profile; close Helium first.")
		return 4
	}
	entries, err := os.ReadDir(a.LogsDir)
	if err != nil {
		fmt.Println("no logs/ directory -- no backups to restore from")
		return 1
	}
	re := regexp.MustCompile(`^(prePull|preImport|preSync)\.(\d{8}-\d{6})$`)
	type backup struct {
		path string
		ts   time.Time
	}
	var backups []backup
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m := re.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		ts, err := time.Parse("20060102-150405", m[2])
		if err == nil {
			backups = append(backups, backup{path: filepath.Join(a.LogsDir, entry.Name()), ts: ts})
		}
	}
	if len(backups) == 0 {
		fmt.Println("no backups found in logs/ (looked for prePull.*, preImport.*, preSync.*)")
		return 1
	}
	sort.Slice(backups, func(i, j int) bool { return backups[i].ts.After(backups[j].ts) })
	latest := backups[0]
	fmt.Printf("restoring from latest backup: %s\n", filepath.Base(latest.path))
	files, _ := os.ReadDir(latest.path)
	restored := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		dst := filepath.Join(a.Profile, file.Name())
		if err := copyFile(filepath.Join(latest.path, file.Name()), dst); err == nil {
			fmt.Printf("  restored %s -> %s\n", file.Name(), dst)
			restored++
		}
	}
	if restored == 0 {
		fmt.Println("no files found in backup directory")
		return 1
	}
	fmt.Printf("\nrestored %d file(s). Launch Helium to see the restored state.\n", restored)
	return 0
}

func (a App) Doctor() int {
	failures, warnings := 0, 0
	report := func(level, name, detail string) {
		if level == "fail" {
			failures++
		} else if level == "warn" {
			warnings++
		}
		fmt.Println(uiStatus(level, name, detail))
	}
	fmt.Println(uiBlock("doctor", "system and repository checks"))
	if out, err := exec.Command("git", "--version").Output(); err == nil {
		report("ok", "git", strings.TrimSpace(string(out)))
	} else {
		report("fail", "git", "git was not found on PATH")
	}
	if out, err := exec.Command("go", "version").Output(); err == nil {
		report("ok", "go", strings.TrimSpace(string(out)))
	} else {
		report("warn", "go", "go was not found on PATH; source builds need Go")
	}
	report("ok", "leveldb", "embedded goleveldb writer")
	if _, err := os.Stat(a.Profile); err == nil {
		report("ok", "profile", a.Profile)
		if _, err := os.Stat(filepath.Join(a.Profile, "Default")); err == nil {
			report("ok", "profile Default", filepath.Join(a.Profile, "Default"))
		} else {
			report("warn", "profile Default", "not found at "+filepath.Join(a.Profile, "Default"))
		}
	} else {
		report("fail", "profile", "not found at "+a.Profile)
	}
	config := DefaultConfig()
	if raw, err := os.ReadFile(config); err == nil {
		repo := ""
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "repo") && strings.Contains(line, "=") {
				repo = strings.Trim(strings.TrimSpace(strings.SplitN(line, "=", 2)[1]), "\"'")
			}
		}
		if repo != "" {
			report("ok", "config", fmt.Sprintf("%s -> %s", config, repo))
		} else {
			report("warn", "config", config+" has no repo value")
		}
	} else {
		report("warn", "config", "not found at "+config)
	}
	if _, err := os.Stat(a.RepoRoot); err == nil {
		report("ok", "repo", a.RepoRoot)
		if _, err := os.Stat(filepath.Join(a.RepoRoot, ".git")); err == nil {
			report("ok", "repo git", ".git exists")
			if remote, err := a.RunGit("remote", "get-url", "origin"); err == nil && strings.TrimSpace(remote) != "" {
				report("ok", "remote", strings.TrimSpace(remote))
			} else {
				report("warn", "remote", "origin is not configured")
			}
		} else {
			report("fail", "repo git", a.RepoRoot+" is not a git repo")
		}
	} else {
		report("fail", "repo", "not found at "+a.RepoRoot)
	}
	if shim, err := exec.LookPath("helium-sync"); err == nil {
		report("warn", "scoop", "shim found at "+shim)
	} else {
		report("warn", "scoop", "not detected")
	}
	fmt.Println()
	if failures > 0 {
		fmt.Println(uiStatus("fail", "doctor", fmt.Sprintf("found %d failure(s) and %d warning(s)", failures, warnings)))
		return 1
	}
	fmt.Println(uiStatus("ok", "doctor", fmt.Sprintf("found 0 failures and %d warning(s)", warnings)))
	return 0
}

func (a App) Version() int {
	fmt.Println(VersionScreen(a.AppVersion(), a.GitRevision(), runtimeSummary()))
	return 0
}

func (a App) Log(n int) int {
	if n <= 0 {
		n = 10
	}
	out, _ := a.RunGit("log", "--max-count="+strconv.Itoa(n), "--pretty=format:%h  %ad  %s", "--date=format:%Y-%m-%d %H:%M")
	if strings.TrimSpace(out) == "" {
		fmt.Println(uiDim.Render("(no commits)"))
		return 0
	}
	fmt.Println(uiBlock("log", strings.TrimSpace(out)))
	return 0
}

func (a App) GC(keepDays int, dryRun bool) int {
	entries, err := os.ReadDir(a.LogsDir)
	if err != nil {
		fmt.Println("no logs/ directory yet -- nothing to gc")
		return 0
	}
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	re := regexp.MustCompile(`^(prePull|preSync)\.(\d{8}-\d{6})$`)
	var old []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m := re.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		ts, err := time.Parse("20060102-150405", m[2])
		if err == nil && ts.Before(cutoff) {
			old = append(old, filepath.Join(a.LogsDir, entry.Name()))
		}
	}
	if len(old) == 0 {
		fmt.Println(uiStatus("ok", "gc", fmt.Sprintf("no backups older than %d day(s) -- nothing to prune", keepDays)))
		return 0
	}
	for _, path := range old {
		fmt.Printf("  %s\n", uiStatus("warn", "gc", filepath.Base(path)))
		if !dryRun {
			_ = os.RemoveAll(path)
		}
	}
	if dryRun {
		fmt.Println(uiDim.Render("(dry run -- pass without --dry-run to actually delete)"))
	} else {
		fmt.Println(uiStatus("ok", "gc", fmt.Sprintf("deleted %d backup directories", len(old))))
	}
	return 0
}

func (a App) Init(target string, force, strict, dryRun bool) int {
	if !force {
		for _, t := range a.Targets {
			if _, err := os.Stat(filepath.Join(a.StateDir, t.StateFilename())); err == nil {
				fmt.Println(uiBlock("init blocked",
					uiBad.Render("canonical state already exists in "+rel(a.RepoRoot, a.StateDir)+"/"),
					"If you really want to overwrite it, pass --force.",
				))
				return 6
			}
		}
	}
	return a.Push(target, strict, dryRun)
}

func (a App) Adopt(yes, allowRunning bool) int {
	if !yes {
		var ok bool
		form := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().Title("Replace local Helium state with canonical repo state?").Value(&ok),
		))
		if err := form.Run(); err != nil || !ok {
			fmt.Println(uiDim.Render("aborted"))
			return 7
		}
	}
	return a.Pull("", allowRunning, false)
}
