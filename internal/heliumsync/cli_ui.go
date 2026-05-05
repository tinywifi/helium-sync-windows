package heliumsync

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	uiBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1)
	uiTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	uiSubtle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	uiDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	uiGood   = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	uiWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	uiBad    = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
)

func UsageScreen() string {
	title := uiTitle.Render("helium-sync")
	subtitle := uiSubtle.Render("Windows fork and Go rewrite")
	commands := []string{
		"setup        configure the repo and profile",
		"push         snapshot live profile into git",
		"pull         restore canonical state into profile",
		"status       compare live and canonical state",
		"diff         print a field-level diff",
		"doctor       check repo, profile, git, and version state",
		"version      show build and runtime details",
		"init         extract live state into canonical state",
		"adopt        replace local state from canonical state",
		"log          show recent sync commits",
		"gc           prune old backup logs",
		"export       write a portable backup JSON",
		"import       apply a portable backup JSON",
		"restore      restore from the latest backup log",
		"completion   print shell completion scripts",
	}
	body := append([]string{
		"usage: helium-sync [--profile PATH] [--repo PATH] <command>",
		"",
		"commands",
	}, commands...)
	return uiBorder.Render(strings.Join([]string{title, subtitle, "", strings.Join(body, "\n")}, "\n"))
}

func VersionScreen(version, revision, runtime string) string {
	head := uiTitle.Render("helium-sync")
	rows := []string{
		renderKV("version", version, "212"),
		renderKV("git", revision, "245"),
		renderKV("go", runtime, "245"),
		renderKV("leveldb", "embedded goleveldb", "245"),
	}
	return uiBorder.Render(strings.Join([]string{head, "", strings.Join(rows, "\n")}, "\n"))
}

func renderKV(label, value, color string) string {
	return fmt.Sprintf("%s: %s",
		uiDim.Render(label),
		lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(value),
	)
}

func uiBlock(title string, lines ...string) string {
	parts := []string{uiTitle.Render(title)}
	if len(lines) > 0 {
		parts = append(parts, "")
		parts = append(parts, lines...)
	}
	return uiBorder.Render(strings.Join(parts, "\n"))
}

func uiStatus(level, name, detail string) string {
	style := uiDim
	switch level {
	case "ok":
		style = uiGood
	case "warn":
		style = uiWarn
	case "fail":
		style = uiBad
	}
	return fmt.Sprintf("%s %-18s %s", style.Render("["+level+"]"), uiDim.Render(name), detail)
}

func uiBullet(name, detail string) string {
	return fmt.Sprintf("%s %s", uiGood.Render("•"), detailWithName(name, detail))
}

func detailWithName(name, detail string) string {
	if name == "" {
		return detail
	}
	return fmt.Sprintf("%s: %s", uiDim.Render(name), detail)
}

func uiSection(title string, lines ...string) string {
	body := append([]string{uiTitle.Render(title)}, lines...)
	return uiBorder.Render(strings.Join(body, "\n"))
}
