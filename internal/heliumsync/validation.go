package heliumsync

import (
	"fmt"
	"strings"
)

func ValidateBookmarks(data any) []string {
	d := asMap(data)
	issues := []string{}
	roots, ok := d["roots"].(map[string]any)
	if !ok {
		return []string{"missing 'roots' key in bookmark data"}
	}
	for _, rk := range []string{"bookmark_bar", "other", "synced"} {
		if _, ok := roots[rk]; !ok {
			issues = append(issues, fmt.Sprintf("missing root '%s' in bookmark data", rk))
		}
	}
	var walk func(map[string]any)
	walk = func(node map[string]any) {
		if str(node["type"]) == "url" {
			url := str(node["url"])
			if url == "" {
				issues = append(issues, fmt.Sprintf("bookmark '%s' has empty URL", str(node["name"])))
			} else if scheme := urlScheme(url); scheme != "" && !allowedScheme(scheme) {
				issues = append(issues, fmt.Sprintf("bookmark '%s': unusual URL scheme '%s'", str(node["name"]), scheme))
			}
		} else if str(node["type"]) == "folder" && str(node["name"]) == "" {
			issues = append(issues, "unnamed folder detected in bookmark tree")
		}
		for _, child := range sliceValue(node["children"]) {
			walk(asMap(child))
		}
	}
	for _, root := range roots {
		walk(asMap(root))
	}
	return issues
}

func ValidateTabGroups(data any) []string {
	d := asMap(data)
	groups := mapValue(d, "groups")
	tabs := mapValue(d, "tabs")
	if _, ok := d["groups"]; !ok {
		return []string{"missing 'groups' or 'tabs' key in tab group data"}
	}
	if _, ok := d["tabs"]; !ok {
		return []string{"missing 'groups' or 'tabs' key in tab group data"}
	}
	var issues []string
	for guid, raw := range groups {
		if str(asMap(raw)["title"]) == "" {
			issues = append(issues, fmt.Sprintf("group '%s' has empty title", short(guid)))
		}
	}
	for guid, raw := range tabs {
		tab := asMap(raw)
		url := str(tab["url"])
		if url == "" {
			issues = append(issues, fmt.Sprintf("tab '%s' has empty URL", short(guid)))
		} else if scheme := urlScheme(url); scheme != "" && !allowedScheme(scheme) {
			issues = append(issues, fmt.Sprintf("tab '%s': unusual URL scheme '%s'", short(guid), scheme))
		}
		groupGUID := str(tab["group_guid"])
		if groupGUID != "" {
			if _, ok := groups[groupGUID]; !ok {
				issues = append(issues, fmt.Sprintf("tab '%s' references non-existent group '%s'", short(guid), short(groupGUID)))
			}
		}
	}
	return issues
}

func urlScheme(url string) string {
	if i := strings.Index(url, "://"); i >= 0 {
		return strings.ToLower(url[:i])
	}
	return ""
}

func allowedScheme(scheme string) bool {
	return scheme == "http" || scheme == "https" || scheme == "chrome" || scheme == "file"
}

func short(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
