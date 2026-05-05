package heliumsync

import "os/exec"

func (a App) RunGit(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", a.RepoRoot}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (a App) hasUpstream() bool {
	_, err := a.RunGit("rev-parse", "--abbrev-ref", "@{upstream}")
	return err == nil
}

func (a App) hasRemote() bool {
	_, err := a.RunGit("remote", "get-url", "origin")
	return err == nil
}
