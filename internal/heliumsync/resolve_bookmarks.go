package heliumsync

type BookmarkResolveItem struct {
	URL        string
	Name       string
	TheirsName string
	Source     string
	Keep       bool
	KeepTheirs bool
	Conflict   bool
}

func MergeBookmarksDefault(live, canonical map[string]any) map[string]any {
	liveURLs := bookmarkNodeByURL(live)
	canonURLs := bookmarkNodeByURL(canonical)
	items := defaultBookmarkSelections(liveURLs, canonURLs)
	roots := map[string]any{}
	for rootKey, rootNode := range mapValue(live, "roots") {
		roots[rootKey] = mergeBookmarkTree(asMap(rootNode), items)
	}
	return map[string]any{
		"roots":    roots,
		"checksum": "",
		"version":  live["version"],
	}
}

func defaultBookmarkSelections(liveURLs, canonURLs map[string]map[string]any) []BookmarkResolveItem {
	var items []BookmarkResolveItem
	for url, node := range liveURLs {
		if _, ok := canonURLs[url]; !ok {
			items = append(items, BookmarkResolveItem{URL: url, Name: str(node["name"]), Source: "local", Keep: true})
		}
	}
	for url, node := range canonURLs {
		if _, ok := liveURLs[url]; !ok {
			items = append(items, BookmarkResolveItem{URL: url, Name: str(node["name"]), Source: "canonical", Keep: true})
		}
	}
	for url, liveNode := range liveURLs {
		canonNode, ok := canonURLs[url]
		if !ok {
			continue
		}
		if str(liveNode["name"]) != str(canonNode["name"]) {
			items = append(items, BookmarkResolveItem{
				URL:        url,
				Name:       str(liveNode["name"]),
				TheirsName: str(canonNode["name"]),
				Source:     "conflict",
				Keep:       true,
				Conflict:   true,
			})
		}
	}
	return items
}

func mergeBookmarkTree(node map[string]any, items []BookmarkResolveItem) map[string]any {
	newNode := cloneMap(node)
	var children []any
	for _, raw := range sliceValue(node["children"]) {
		child := asMap(raw)
		if str(child["type"]) == "url" {
			url := str(child["url"])
			item, ok := bookmarkItemByURL(items, url)
			if !ok {
				children = append(children, child)
			} else if item.Source == "local" && item.Keep {
				children = append(children, child)
			} else if item.Source == "canonical" && item.Keep {
				children = append(children, child)
			} else if item.Source == "conflict" {
				if item.KeepTheirs {
					copy := cloneMap(child)
					copy["name"] = item.TheirsName
					children = append(children, copy)
				} else if item.Keep {
					children = append(children, child)
				}
			}
		} else if str(child["type"]) == "folder" {
			merged := mergeBookmarkTree(child, items)
			if len(sliceValue(merged["children"])) > 0 {
				children = append(children, merged)
			}
		}
	}
	newNode["children"] = children
	return newNode
}

func bookmarkItemByURL(items []BookmarkResolveItem, url string) (BookmarkResolveItem, bool) {
	for _, item := range items {
		if item.URL == url {
			return item, true
		}
	}
	return BookmarkResolveItem{}, false
}

func bookmarkNodeByURL(tree map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, node := range WalkURLs(tree) {
		out[str(node["url"])] = node
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
