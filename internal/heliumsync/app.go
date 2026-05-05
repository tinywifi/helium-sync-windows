package heliumsync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	line := fmt.Sprintf("%s %s", nowISO(), fmt.Sprintf(format, args...))
	fmt.Println(renderLogLine(line))
	_ = os.MkdirAll(a.LogsDir, 0755)
	f, err := os.OpenFile(filepath.Join(a.LogsDir, "sync.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		_, _ = f.WriteString(line + "\n")
	}
	clog.Infof(format, args...)
}

func renderLogLine(line string) string {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 {
		return uiDim.Render(line)
	}
	return fmt.Sprintf("%s %s", uiDim.Render(parts[0]), parts[1])
}
