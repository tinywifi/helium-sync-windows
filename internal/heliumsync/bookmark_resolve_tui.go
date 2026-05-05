package heliumsync

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type bookmarkResolveRow struct {
	Key              string
	Path             string
	URL              string
	LocalName        string
	CanonicalName    string
	LocalPresent     bool
	CanonicalPresent bool
	Selected         string
}

type bookmarkResolveItem struct {
	row bookmarkResolveRow
}

func (i bookmarkResolveItem) FilterValue() string {
	return strings.Join([]string{i.row.URL, i.row.Path, i.row.LocalName, i.row.CanonicalName, i.row.Selected}, " ")
}

func (i bookmarkResolveItem) Title() string       { return i.row.URL }
func (i bookmarkResolveItem) Description() string { return i.row.Path }

type bookmarkResolveDelegate struct{}

func (d bookmarkResolveDelegate) Height() int                               { return 3 }
func (d bookmarkResolveDelegate) Spacing() int                              { return 1 }
func (d bookmarkResolveDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d bookmarkResolveDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(bookmarkResolveItem)
	if !ok {
		return
	}
	row := it.row
	selected := m.Index() == index

	cursor := " "
	if selected {
		cursor = ">"
	}
	badge := renderBookmarkBadge(row.Selected)
	if row.LocalPresent && row.CanonicalPresent && row.LocalName != row.CanonicalName {
		badge = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true).Render(row.Selected)
	}

	header := fmt.Sprintf("%s %s %s", cursor, badge, clipText(row.URL, 72))
	localLine := fmt.Sprintf("local: %s", clipText(bookmarkName(row.LocalPresent, row.LocalName), 28))
	canonLine := fmt.Sprintf("canonical: %s", clipText(bookmarkName(row.CanonicalPresent, row.CanonicalName), 28))
	pathLine := fmt.Sprintf("path: %s", clipText(row.Path, 44))

	headerStyle := lipgloss.NewStyle().Bold(true)
	if selected {
		headerStyle = headerStyle.Foreground(lipgloss.Color("212"))
	}

	body := strings.Join([]string{
		headerStyle.Render(header),
		styleBookmarkSide("local", localLine, row.LocalPresent && row.Selected == "local"),
		styleBookmarkSide("canonical", canonLine, row.CanonicalPresent && row.Selected == "canonical"),
		styleBookmarkPath(pathLine, selected),
	}, "\n")

	if selected {
		body = lipgloss.NewStyle().BorderLeft(true).BorderForeground(lipgloss.Color("212")).PaddingLeft(1).Render(body)
	} else {
		body = lipgloss.NewStyle().PaddingLeft(1).Render(body)
	}
	fmt.Fprint(w, body)
}

func styleBookmarkSide(label, text string, active bool) string {
	style := lipgloss.NewStyle()
	if active {
		style = style.Foreground(lipgloss.Color("42")).Bold(true)
	} else {
		style = style.Foreground(lipgloss.Color("240"))
	}
	return style.Render(text)
}

func styleBookmarkPath(text string, selected bool) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	if selected {
		style = style.Foreground(lipgloss.Color("252"))
	}
	return style.Render(text)
}

func renderBookmarkBadge(selected string) string {
	switch selected {
	case "canonical":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("69")).Bold(true).Render("[canonical]")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render("[local]")
	}
}

func bookmarkName(present bool, name string) string {
	if !present || strings.TrimSpace(name) == "" {
		return "<missing>"
	}
	return name
}

func clipText(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width <= 1 {
		return string(r[:width])
	}
	return string(r[:width-1]) + "…"
}

func bookmarkResolveRows(live, canonical map[string]any) []bookmarkResolveRow {
	liveNodes, _ := flattenBookmarks(live)
	canonNodes, _ := flattenBookmarks(canonical)
	keys := make([]string, 0, len(liveNodes)+len(canonNodes))
	seen := map[string]bool{}
	for key := range liveNodes {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	for key := range canonNodes {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	rows := make([]bookmarkResolveRow, 0, len(keys))
	for _, key := range keys {
		path, url := bookmarkKeyParts(key)
		row := bookmarkResolveRow{Key: key, Path: path, URL: url}
		if node, ok := liveNodes[key]; ok {
			row.LocalPresent = true
			row.LocalName = str(node["name"])
		}
		if node, ok := canonNodes[key]; ok {
			row.CanonicalPresent = true
			row.CanonicalName = str(node["name"])
		}
		switch {
		case row.LocalPresent && !row.CanonicalPresent:
			row.Selected = "local"
		case row.CanonicalPresent && !row.LocalPresent:
			row.Selected = "canonical"
		default:
			row.Selected = "local"
		}
		rows = append(rows, row)
	}
	return rows
}

func bookmarkKeyParts(key string) (string, string) {
	parts := strings.Split(key, "\x00")
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

type bookmarkResolveModel struct {
	list     list.Model
	rows     []bookmarkResolveRow
	canceled bool
}

func newBookmarkResolveModel(rows []bookmarkResolveRow) bookmarkResolveModel {
	items := make([]list.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, bookmarkResolveItem{row: row})
	}
	l := list.New(items, bookmarkResolveDelegate{}, 112, 18)
	l.Title = "Resolve bookmarks divergences"
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	return bookmarkResolveModel{list: l, rows: rows}
}

func (m bookmarkResolveModel) Init() tea.Cmd { return nil }

func (m bookmarkResolveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return m, tea.Quit
		case "space":
			return m.toggleSelected()
		case "ctrl+c", "q":
			m.canceled = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m bookmarkResolveModel) toggleSelected() (tea.Model, tea.Cmd) {
	idx := m.list.GlobalIndex()
	if idx < 0 || idx >= len(m.rows) {
		return m, nil
	}
	row := m.rows[idx]
	if row.LocalPresent && row.CanonicalPresent {
		if row.Selected == "local" {
			row.Selected = "canonical"
		} else {
			row.Selected = "local"
		}
	} else if row.LocalPresent {
		row.Selected = "local"
	} else {
		row.Selected = "canonical"
	}
	m.rows[idx] = row
	m.list.SetItem(idx, bookmarkResolveItem{row: row})
	return m, nil
}

func (m bookmarkResolveModel) View() tea.View {
	shell := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Resolve bookmarks divergences")
	help := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("space toggles local/canonical • enter accepts • q cancels")
	return tea.NewView(shell.Render(fmt.Sprintf("%s\n\n%s\n\n%s", title, m.list.View(), help)))
}

func mergeBookmarksFromRows(live, canonical map[string]any, rows []bookmarkResolveRow) map[string]any {
	merged := cloneMap(live)
	roots := asMap(merged["roots"])
	canonRoots := asMap(canonical["roots"])
	for _, row := range rows {
		if row.Selected != "canonical" {
			continue
		}
		rootKey, folderPath := bookmarkRootAndFolderPath(row.Path)
		mergedRoot := ensureBookmarkRoot(roots, canonRoots, rootKey)
		canonRoot := asMap(canonRoots[rootKey])
		canonNode, ok := bookmarkURLNodeAtPath(canonRoot, folderPath, row.URL)
		if !ok {
			continue
		}
		parent := ensureBookmarkFolderPath(mergedRoot, folderPath)
		if bookmarkReplaceURLChild(parent, row.URL, canonNode) {
			continue
		}
		children := sliceValue(parent["children"])
		children = append(children, cloneMap(canonNode))
		parent["children"] = children
	}
	return merged
}

func bookmarkRootAndFolderPath(path string) (string, string) {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], "/")
}

func ensureBookmarkRoot(roots, canonRoots map[string]any, rootKey string) map[string]any {
	if root, ok := roots[rootKey]; ok {
		copy := cloneMap(asMap(root))
		if children := sliceValue(copy["children"]); children != nil {
			copy["children"] = append([]any(nil), children...)
		}
		roots[rootKey] = copy
		return copy
	}
	if root, ok := canonRoots[rootKey]; ok {
		copy := cloneMap(asMap(root))
		if children := sliceValue(copy["children"]); children != nil {
			copy["children"] = append([]any(nil), children...)
		}
		roots[rootKey] = copy
		return copy
	}
	root := map[string]any{"type": "folder", "name": rootKey, "children": []any{}}
	roots[rootKey] = root
	return root
}

func ensureBookmarkFolderPath(root map[string]any, folderPath string) map[string]any {
	current := root
	if folderPath == "" {
		return current
	}
	for _, name := range strings.Split(folderPath, "/") {
		next := bookmarkFolderChild(current, name)
		if next == nil {
			next = map[string]any{"type": "folder", "name": name, "children": []any{}}
			children := append([]any(nil), sliceValue(current["children"])...)
			children = append(children, next)
			current["children"] = children
		}
		current = next
	}
	return current
}

func bookmarkFolderChild(node map[string]any, name string) map[string]any {
	for _, raw := range sliceValue(node["children"]) {
		child := asMap(raw)
		if str(child["type"]) == "folder" && str(child["name"]) == name {
			return child
		}
	}
	return nil
}

func bookmarkURLNodeAtPath(root map[string]any, folderPath, url string) (map[string]any, bool) {
	folder := ensureBookmarkFolderPath(cloneMap(root), folderPath)
	for _, raw := range sliceValue(folder["children"]) {
		child := asMap(raw)
		if str(child["type"]) == "url" && str(child["url"]) == url {
			return cloneMap(child), true
		}
	}
	return nil, false
}

func bookmarkReplaceURLChild(folder map[string]any, url string, node map[string]any) bool {
	children := append([]any(nil), sliceValue(folder["children"])...)
	for i, raw := range children {
		child := asMap(raw)
		if str(child["type"]) == "url" && str(child["url"]) == url {
			children[i] = cloneMap(node)
			folder["children"] = children
			return true
		}
	}
	return false
}
