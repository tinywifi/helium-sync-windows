package heliumsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/harmonica"
	tree "github.com/rpiawesomeness/bubble-tree"
)

func (a App) Resolve(target, theirs string) int {
	if target == "" {
		target = "bookmarks"
	}
	if target != "bookmarks" && target != "saved_tab_groups" {
		fmt.Printf("resolve: unknown target '%s' (choose: bookmarks, saved_tab_groups)\n", target)
		return 1
	}
	return a.resolveTUI(target, theirs)
}

func (a App) resolveTUI(target, theirs string) int {
	canonical, rc := a.loadCanonical(target, theirs)
	if rc != 0 {
		return rc
	}
	var tgt Target
	for _, t := range a.Targets {
		if t.Name() == target {
			tgt = t
		}
	}
	if tgt == nil {
		return 2
	}
	live, err := tgt.Extract(a.Profile)
	if err != nil {
		fmt.Printf("cannot extract live %s: %v\n", target, err)
		return 2
	}
	if tgt.SemanticallyEqual(live, canonical) {
		fmt.Println("no divergences found -- live and canonical are identical")
		return 0
	}
	if os.Getenv("CI") != "" || !isTerminal() {
		fmt.Println("(resolve requires an interactive terminal)")
		fmt.Println("Run: helium-sync pull --dry-run   to see what would change")
		return 1
	}
	items := []list.Item{
		resolveItem{"local", tgt.Summary(live)},
		resolveItem{"canonical", tgt.Summary(canonical)},
	}
	l := list.New(items, list.NewDefaultDelegate(), 72, 12)
	l.Title = "Resolve " + target + " divergences"
	tv := tree.New(resolveTreeNodes(target, live, canonical), 72, 12, &tree.TreeOptions{
		ShowHelp:          true,
		HighlightFullLine: true,
		ChildPrefix:       tree.Smooth,
	})
	if _, err := tea.NewProgram(resolveModel{list: l, tree: tv}).Run(); err != nil {
		return 7
	}
	merged := defaultResolvedState(target, live, canonical)
	backup := filepath.Join(a.LogsDir, "preSync."+nowStamp())
	if err := tgt.Apply(a.Profile, merged, backup); err != nil {
		fmt.Printf("failed to write merged %s: %v\n", target, err)
		return 2
	}
	fmt.Printf("resolved: kept local %s\n", target)
	fmt.Printf("backup: %s\n", rel(a.RepoRoot, backup))
	return 0
}

func defaultResolvedState(target string, live, canonical any) any {
	if target == "saved_tab_groups" {
		liveMap, canonMap := asMap(live), asMap(canonical)
		divs := TabGroupDivergencesOf(liveMap, canonMap)
		var selections []ResolveSelection
		for guid := range divs.LiveOnlyGroups {
			selections = append(selections, ResolveSelection{Type: "group", GUID: guid, Source: "local", Keep: true})
		}
		for guid := range divs.CanonOnlyGroups {
			selections = append(selections, ResolveSelection{Type: "group", GUID: guid, Source: "canonical", Keep: true})
		}
		for guid := range divs.ConflictGroups {
			selections = append(selections, ResolveSelection{Type: "group", GUID: guid, Source: "conflict", Keep: true})
		}
		for guid := range divs.LiveOnlyTabs {
			selections = append(selections, ResolveSelection{Type: "tab", GUID: guid, Source: "local", Keep: true})
		}
		for guid := range divs.CanonOnlyTabs {
			selections = append(selections, ResolveSelection{Type: "tab", GUID: guid, Source: "canonical", Keep: true})
		}
		for guid := range divs.ConflictTabs {
			selections = append(selections, ResolveSelection{Type: "tab", GUID: guid, Source: "conflict", Keep: true})
		}
		return MergeTabGroups(liveMap, canonMap, selections)
	}
	if target == "bookmarks" {
		return MergeBookmarksDefault(asMap(live), asMap(canonical))
	}
	return live
}

func (a App) loadCanonical(target, theirs string) (any, int) {
	var canonical any
	if theirs != "" {
		raw, err := os.ReadFile(abs(expandHome(theirs)))
		if err != nil {
			fmt.Printf("file not found: %s\n", theirs)
			return nil, 1
		}
		_ = json.Unmarshal(raw, &canonical)
		return canonical, 0
	}
	filename := "bookmarks.json"
	if target == "saved_tab_groups" {
		filename = "saved_tab_groups.json"
	}
	raw, err := os.ReadFile(filepath.Join(a.StateDir, filename))
	if err != nil {
		fmt.Printf("no canonical %s state -- nothing to resolve against\n", target)
		return nil, 1
	}
	_ = json.Unmarshal(raw, &canonical)
	return canonical, 0
}

type resolveItem struct {
	title string
	desc  string
}

func (i resolveItem) FilterValue() string { return i.title }
func (i resolveItem) Title() string       { return i.title }
func (i resolveItem) Description() string { return i.desc }

type resolveModel struct {
	list list.Model
	tree tree.Model
}

func (m resolveModel) Init() tea.Cmd { return nil }

func (m resolveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" || msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.tree, _ = m.tree.Update(msg)
	return m, cmd
}

func (m resolveModel) View() tea.View {
	shell := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Resolve preview")
	panel := lipgloss.NewStyle().Padding(0, 1)
	spring := harmonica.NewSpring(harmonica.FPS(60), 6.0, 0.7)
	pos, _ := spring.Update(0, 0, 1)
	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		panel.Render(m.tree.View()),
		panel.Render(m.list.View()),
	)
	return tea.NewView(shell.Render(fmt.Sprintf(
		"%s %.0f%%\n\n%s\n\nEnter accepts the current local-first merge. q cancels.",
		title,
		pos*100,
		body,
	)))
}

func resolveTreeNodes(target string, live, canonical any) []tree.Node {
	return []tree.Node{
		{
			Value: "resolve",
			Desc:  target,
			Children: []tree.Node{
				{Value: "local", Desc: summaryForTree(target, live)},
				{Value: "canonical", Desc: summaryForTree(target, canonical)},
			},
		},
	}
}

func summaryForTree(target string, data any) string {
	if target == "bookmarks" {
		return fmt.Sprintf("%d URLs", len(WalkURLs(asMap(data))))
	}
	if target == "saved_tab_groups" {
		m := asMap(data)
		return fmt.Sprintf("%d groups, %d tabs", len(mapValue(m, "groups")), len(mapValue(m, "tabs")))
	}
	return "state"
}
