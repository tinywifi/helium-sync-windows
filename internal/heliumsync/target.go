package heliumsync

type Target interface {
	Name() string
	StateFilename() string
	Extract(profile string) (any, error)
	Apply(profile string, data any, backupDir string) error
	Serialize(data any) (string, error)
	Deserialize(text string) (any, error)
	SemanticallyEqual(a, b any) bool
	Summary(data any) string
}

func Targets() []Target {
	return []Target{Bookmarks{}, SavedTabGroups{}}
}

func validationIssues(t Target, data any) []string {
	switch t.Name() {
	case "bookmarks":
		return ValidateBookmarks(data)
	case "saved_tab_groups":
		return ValidateTabGroups(data)
	default:
		return nil
	}
}
