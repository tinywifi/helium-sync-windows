package heliumsync

import (
	"os"
	"path/filepath"

	clog "charm.land/log/v2"
)

type App struct {
	RepoRoot string
	StateDir string
	LogsDir  string
	Profile  string
	Targets  []Target
}

func New(repo, profile string) App {
	if repo == "" {
		repo = ResolveRepo("", os.Environ(), DefaultConfig())
	}
	if profile == "" {
		profile = DefaultProfile()
	}
	return App{
		RepoRoot: repo,
		StateDir: filepath.Join(repo, "state"),
		LogsDir:  filepath.Join(repo, "logs"),
		Profile:  profile,
		Targets:  Targets(),
	}
}

func (a App) targetByName(name string) []Target {
	if name == "" {
		return a.Targets
	}
	var out []Target
	for _, t := range a.Targets {
		if t.Name() == name {
			out = append(out, t)
		}
	}
	return out
}

func (a App) logf(format string, args ...any) {
	clog.Infof(format, args...)
}


