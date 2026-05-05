package heliumsync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a App) Push(target string, strict, dryRun bool) int {
	a.logf("=== push from %s ===", Hostname())
	if dryRun {
		return a.pushDryRun(target)
	}
	hasUpstream := a.hasUpstream()
	if hasUpstream {
		a.logf("git pull --rebase...")
		if out, err := a.RunGit("pull", "--rebase", "--quiet"); err != nil {
			a.logf("git pull failed: %s", strings.TrimSpace(out))
			return 1
		}
	} else {
		a.logf("(no upstream configured -- skipping git pull)")
	}

	_ = os.MkdirAll(a.StateDir, 0755)
	changed := false
	for _, t := range a.targetByName(target) {
		a.logf("  extract %s...", t.Name())
		data, err := t.Extract(a.Profile)
		if err != nil {
			a.logf("  extract %s failed: %v", t.Name(), err)
			return 2
		}
		issues := validationIssues(t, data)
		for _, issue := range issues {
			a.logf("  %s WARNING: %s", t.Name(), issue)
		}
		if strict && len(issues) > 0 {
			a.logf("  %s: validation failed (--strict mode)", t.Name())
			return 9
		}
		text, err := t.Serialize(data)
		if err != nil {
			a.logf("  serialize %s failed: %v", t.Name(), err)
			return 2
		}
		stateFile := filepath.Join(a.StateDir, t.StateFilename())
		prev, _ := os.ReadFile(stateFile)
		if string(prev) == text {
			a.logf("  %s: unchanged", t.Name())
			continue
		}
		_ = os.MkdirAll(filepath.Dir(stateFile), 0755)
		if err := os.WriteFile(stateFile, []byte(text), 0644); err != nil {
			a.logf("  write %s failed: %v", t.Name(), err)
			return 2
		}
		changed = true
		a.logf("  %s: wrote %d bytes (%s)", t.Name(), len(text), t.Summary(data))
	}
	if !changed {
		a.logf("no changes to push")
		if !a.hasUpstream() && a.hasRemote() {
			if out, err := a.RunGit("push", "-u", "origin", "main", "--quiet"); err != nil {
				a.logf("push failed: %s", strings.TrimSpace(out))
				return 3
			}
			a.logf("pushed to origin (upstream now set)")
		}
		return 0
	}
	if _, err := a.RunGit("add", "state/"); err != nil {
		return 3
	}
	if _, err := a.RunGit("diff", "--staged", "--quiet"); err == nil {
		a.logf("nothing actually staged after add")
		return 0
	}
	msg := fmt.Sprintf("push from %s %s", Hostname(), nowISO())
	if out, err := a.RunGit("commit", "-q", "-m", msg); err != nil {
		a.logf("commit failed: %s", strings.TrimSpace(out))
		return 3
	}
	a.logf("committed: %s", msg)
	if !hasUpstream {
		if !a.hasRemote() {
			a.logf("(no remote configured -- local commit only; add a remote with: git -C <repo> remote add origin <url>)")
			return 0
		}
		if out, err := a.RunGit("push", "-u", "origin", "main", "--quiet"); err != nil {
			a.logf("push failed: %s", strings.TrimSpace(out))
			return 3
		}
		a.logf("pushed to origin (upstream now set)")
		return 0
	}
	if out, err := a.RunGit("push", "--quiet"); err != nil {
		a.logf("push rejected: %s", strings.TrimSpace(out))
		if _, err := a.RunGit("pull", "--rebase", "--quiet"); err == nil {
			if _, err := a.RunGit("push", "--quiet"); err == nil {
				a.logf("pushed on retry")
				return 0
			}
		}
		return 3
	}
	a.logf("pushed to origin")
	return 0
}

func (a App) Pull(target string, allowRunning, dryRun bool) int {
	a.logf("=== pull on %s ===", Hostname())
	if dryRun {
		return a.pullDryRun(target)
	}
	if !allowRunning && HeliumRunning() {
		fmt.Println("ERROR: Helium is running on this device.")
		fmt.Println("Pull writes directly to your Helium profile; close Helium first.")
		fmt.Println("(Or, for testing: --allow-helium-running)")
		return 4
	}
	if a.hasUpstream() {
		a.logf("git pull --rebase...")
		if out, err := a.RunGit("pull", "--rebase", "--quiet"); err != nil {
			a.logf("git pull failed: %s", strings.TrimSpace(out))
			return 1
		}
	} else {
		a.logf("(no upstream configured -- nothing to pull from remote)")
	}
	backup := filepath.Join(a.LogsDir, "prePull."+nowStamp())
	_ = os.MkdirAll(backup, 0755)
	applied := 0
	for _, t := range a.targetByName(target) {
		raw, err := os.ReadFile(filepath.Join(a.StateDir, t.StateFilename()))
		if err != nil {
			a.logf("  %s: no canonical state in repo -- skipping (run init on the source device)", t.Name())
			continue
		}
		data, err := t.Deserialize(string(raw))
		if err != nil {
			a.logf("  %s: deserialize failed: %v", t.Name(), err)
			return 5
		}
		a.logf("  apply %s...", t.Name())
		if err := t.Apply(a.Profile, data, backup); err != nil {
			a.logf("  apply %s failed: %v", t.Name(), err)
			return 5
		}
		a.logf("  %s: applied (%s)", t.Name(), t.Summary(data))
		applied++
	}
	a.logf("backup: %s", rel(a.RepoRoot, backup))
	a.logf("done. %d target(s) applied. Launch Helium to see the synced state.", applied)
	return 0
}

func (a App) pushDryRun(target string) int {
	a.logf("=== push dry-run from %s ===", Hostname())
	anyChange := false
	for _, t := range a.targetByName(target) {
		data, err := t.Extract(a.Profile)
		if err != nil {
			a.logf("  extract %s failed: %v", t.Name(), err)
			return 2
		}
		text, _ := t.Serialize(data)
		prev, _ := os.ReadFile(filepath.Join(a.StateDir, t.StateFilename()))
		if string(prev) != text {
			anyChange = true
			a.logf("  %s: WOULD UPDATE (%s)", t.Name(), t.Summary(data))
		} else {
			a.logf("  %s: unchanged", t.Name())
		}
	}
	if !anyChange {
		a.logf("no changes to push")
	} else {
		a.logf("would commit and push changes")
	}
	fmt.Println("(dry run -- pass without --dry-run to actually commit and push)")
	return 0
}

func (a App) pullDryRun(target string) int {
	a.logf("=== pull dry-run on %s ===", Hostname())
	a.logf("would: git pull --rebase")
	n := 0
	for _, t := range a.targetByName(target) {
		raw, err := os.ReadFile(filepath.Join(a.StateDir, t.StateFilename()))
		if err != nil {
			a.logf("  %s: no canonical state -- would skip", t.Name())
			continue
		}
		data, _ := t.Deserialize(string(raw))
		a.logf("  %s: would apply (%s)", t.Name(), t.Summary(data))
		n++
	}
	if n == 0 {
		a.logf("nothing to pull")
	} else {
		a.logf("would apply %d target(s)", n)
		a.logf("would backup current profile to logs/prePull.<timestamp>/")
	}
	fmt.Println("(dry run -- pass without --dry-run to actually pull and write to profile)")
	return 0
}
