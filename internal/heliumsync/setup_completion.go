package heliumsync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
)

func (a App) Setup(force, yes bool) int {
	config := DefaultConfig()
	if _, err := os.Stat(config); err == nil && !force {
		fmt.Printf("Already configured: %s\n", config)
		if repo := configRepo(config); repo != "" {
			fmt.Printf("  data repo: %s\n", repo)
		}
		fmt.Println()
		fmt.Println("Pass --force to reconfigure (existing config will be overwritten).")
		return 8
	}
	home, _ := os.UserHomeDir()
	repo := filepath.Join(home, "helium-data")
	remote := ""
	if !yes && isTerminal() {
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Data repository").Value(&repo),
			huh.NewInput().Title("Remote URL").Placeholder("leave empty for first device").Value(&remote),
		))
		if err := form.Run(); err != nil {
			return 7
		}
	}
	if entries, err := os.ReadDir(repo); err == nil && len(entries) > 0 {
		if _, err := os.Stat(filepath.Join(repo, ".git")); err != nil {
			fmt.Printf("  [error] %s exists and isn't empty / isn't a git repo. Pick a different path.\n", repo)
			return 8
		}
	}
	_ = os.MkdirAll(repo, 0755)
	if _, err := os.Stat(filepath.Join(repo, ".git")); err != nil {
		_ = exec.Command("git", "-C", repo, "init", "-q", "-b", "main").Run()
		fmt.Println("  [ok] initialized git repo")
	}
	isAdopt := false
	if remote != "" {
		if exec.Command("git", "-C", repo, "remote", "get-url", "origin").Run() == nil {
			_ = exec.Command("git", "-C", repo, "remote", "set-url", "origin", remote).Run()
		} else {
			_ = exec.Command("git", "-C", repo, "remote", "add", "origin", remote).Run()
		}
		fmt.Printf("  [ok] origin -> %s\n", remote)
		if out, err := exec.Command("git", "-C", repo, "ls-remote", "origin", "main").Output(); err == nil && strings.TrimSpace(string(out)) != "" {
			isAdopt = true
			fmt.Println("  [ok] remote has existing canonical state - this is an ADOPT situation")
			_ = exec.Command("git", "-C", repo, "fetch", "origin", "main", "--quiet").Run()
			_ = exec.Command("git", "-C", repo, "checkout", "-q", "-B", "main", "origin/main").Run()
		} else {
			fmt.Println("  [ok] remote is empty - first-device flow")
		}
	}
	_ = os.MkdirAll(filepath.Dir(config), 0755)
	_ = os.WriteFile(config, []byte(fmt.Sprintf("repo = %q\n", filepath.ToSlash(repo))), 0644)
	fmt.Printf("  [ok] %s\n", config)
	next := New(repo, a.Profile)
	if isAdopt {
		fmt.Println("Next: ADOPT - this will REPLACE your local Helium state with the canonical")
		fmt.Println("from the remote. Backups go to logs/prePull.<timestamp>/.")
		if HeliumRunning() {
			fmt.Println("WARNING: Helium is running. Close it first, then run:")
			fmt.Println("    helium-sync adopt")
			return 0
		}
		if !yes && !confirm("Continue?", false) {
			fmt.Println("aborted. When you're ready, run: helium-sync adopt")
			return 0
		}
		return next.Pull("", false, false)
	}
	fmt.Println("Next: extract your current Helium state and push to canonical.")
	if !yes && !confirm("Continue?", true) {
		fmt.Println("aborted. When you're ready, run: helium-sync init")
		return 0
	}
	rc := next.Push("", false, false)
	if rc == 0 {
		fmt.Println()
		fmt.Println("Done. On other devices: install helium-sync, run `helium-sync setup` and")
		fmt.Println("give it the remote URL pointing at this data repo.")
	}
	return rc
}

func configRepo(config string) string {
	raw, err := os.ReadFile(config)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "repo") && strings.Contains(line, "=") {
			return strings.Trim(strings.TrimSpace(strings.SplitN(line, "=", 2)[1]), "\"'")
		}
	}
	return ""
}

func confirm(title string, defaultValue bool) bool {
	if !isTerminal() {
		return defaultValue
	}
	value := defaultValue
	form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title(title).Value(&value)))
	if err := form.Run(); err != nil {
		return defaultValue
	}
	return value
}

func (a App) Completion(shell string) int {
	subs := []string{"setup", "push", "pull", "status", "diff", "doctor", "version", "log", "gc", "init", "adopt", "export", "import", "restore", "resolve", "completion"}
	flags := map[string][]string{
		"setup":      {"--force", "--profile", "--repo"},
		"push":       {"--target", "--strict", "--dry-run", "--profile", "--repo"},
		"pull":       {"--allow-helium-running", "--target", "--dry-run", "--profile", "--repo"},
		"status":     {"--target", "--profile", "--repo"},
		"diff":       {"--target", "--profile", "--repo"},
		"doctor":     {"--profile", "--repo"},
		"log":        {"-n", "--profile", "--repo"},
		"gc":         {"--keep-days", "--dry-run", "--profile", "--repo"},
		"init":       {"--force", "--target", "--profile", "--repo"},
		"adopt":      {"--yes", "-y", "--allow-helium-running", "--profile", "--repo"},
		"export":     {"--output", "--target", "--profile", "--repo"},
		"import":     {"--allow-helium-running", "--target", "--profile", "--repo"},
		"restore":    {"--allow-helium-running", "--profile", "--repo"},
		"resolve":    {"--target", "--theirs", "--profile", "--repo"},
		"completion": {"--shell", "--profile", "--repo"},
	}
	if shell == "cmd" {
		fmt.Println("@echo off")
		fmt.Println("REM helium-sync DOSKEY macros for cmd.exe")
		for _, sub := range subs {
			fmt.Printf("doskey helium-sync %s=helium-sync %s $*\n", sub, sub)
		}
		return 0
	}
	fmt.Println("$scriptBlock = {")
	fmt.Println("  param($wordToComplete, $commandAst, $cursorPosition)")
	fmt.Println("  $commandElements = $commandAst.CommandElements")
	fmt.Printf("  $subcommands = @(%s)\n", quoteList(subs))
	fmt.Println("  if ($commandElements.Count -le 2) {")
	fmt.Println("    $subcommands | Where-Object { $_ -like \"$wordToComplete*\" }")
	fmt.Println("    return")
	fmt.Println("  }")
	fmt.Println("  $subcommand = $commandElements[2].Value")
	fmt.Println("  $flags = @{")
	for _, sub := range subs {
		fmt.Printf("    %q = @(%s)\n", sub, quoteList(flags[sub]))
	}
	fmt.Println("  }")
	fmt.Println("  if ($flags.ContainsKey($subcommand)) {")
	fmt.Println("    $flags[$subcommand] | Where-Object { $_ -like \"$wordToComplete*\" }")
	fmt.Println("  }")
	fmt.Println("}")
	fmt.Println("Register-ArgumentCompleter -CommandName helium-sync -ScriptBlock $scriptBlock")
	fmt.Println("Register-ArgumentCompleter -CommandName helium-sync.bat -ScriptBlock $scriptBlock")
	return 0
}

func quoteList(items []string) string {
	quoted := make([]string, 0, len(items))
	for _, item := range items {
		quoted = append(quoted, fmt.Sprintf("%q", item))
	}
	return strings.Join(quoted, ", ")
}
