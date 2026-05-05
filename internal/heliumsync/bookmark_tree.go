package heliumsync

import "fmt"

func WalkURLs(tree map[string]any) []map[string]any {
	var out []map[string]any
	for _, root := range mapValue(asMap(tree["roots"]), "") {
		out = walkURLsRec(asMap(root), out)
	}
	return out
}

func walkURLsRec(node map[string]any, out []map[string]any) []map[string]any {
	for _, raw := range sliceValue(node["children"]) {
		child := asMap(raw)
		if str(child["type"]) == "url" {
			out = append(out, child)
		} else if str(child["type"]) == "folder" {
			out = walkURLsRec(child, out)
		}
	}
	return out
}

func flattenBookmarks(tree map[string]any) (map[string]map[string]any, map[string]bool) {
	urls := map[string]map[string]any{}
	folders := map[string]bool{}
	roots := asMap(tree["roots"])
	for _, rootKey := range []string{"bookmark_bar", "other", "synced"} {
		if root, ok := roots[rootKey]; ok {
			folders[rootKey] = true
			flattenWalk(asMap(root), rootKey, urls, folders)
		}
	}
	return urls, folders
}

func flattenWalk(node map[string]any, path string, urls map[string]map[string]any, folders map[string]bool) {
	for _, raw := range sliceValue(node["children"]) {
		child := asMap(raw)
		switch str(child["type"]) {
		case "url":
			key := path + "\x00" + str(child["url"])
			if _, ok := urls[key]; ok {
				for i := 2; ; i++ {
					candidate := fmt.Sprintf("%s\x00%s\x00%d", path, str(child["url"]), i)
					if _, ok := urls[candidate]; !ok {
						key = candidate
						break
					}
				}
			}
			urls[key] = child
		case "folder":
			childPath := path + "/" + str(child["name"])
			if folders[childPath] {
				for i := 2; ; i++ {
					candidate := fmt.Sprintf("%s/%s#%d", path, str(child["name"]), i)
					if !folders[candidate] {
						childPath = candidate
						break
					}
				}
			}
			folders[childPath] = true
			flattenWalk(child, childPath, urls, folders)
		}
	}
}
