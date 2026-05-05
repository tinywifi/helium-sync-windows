package heliumsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func (a App) Export(output, target string) int {
	targets := a.targetByName(target)
	if target != "" && len(targets) == 0 {
		fmt.Printf("  [error] unknown target: %s\n", target)
		return 1
	}
	exports := map[string]any{}
	for _, t := range targets {
		raw, err := os.ReadFile(filepath.Join(a.StateDir, t.StateFilename()))
		if err != nil {
			fmt.Printf("  %s: no canonical state to export (run `helium-sync init` first)\n", t.Name())
			continue
		}
		data, err := t.Deserialize(string(raw))
		if err != nil {
			return 1
		}
		exports[t.Name()] = map[string]any{"format_version": 1, "target": t.Name(), "data": data}
	}
	if len(exports) == 0 {
		fmt.Println("  nothing to export")
		return 1
	}
	payload := map[string]any{
		"exported_at":   nowISO(),
		"exported_from": Hostname(),
		"targets":       exports,
	}
	if output == "" {
		output = filepath.Join(a.RepoRoot, "helium-sync-export-"+nowStamp()+".json")
	}
	output = abs(expandHome(output))
	_ = os.MkdirAll(filepath.Dir(output), 0755)
	raw, _ := json.MarshalIndent(payload, "", "  ")
	_ = os.WriteFile(output, raw, 0644)
	fmt.Printf("  exported %d target(s) to %s\n", len(exports), output)
	for name := range exports {
		fmt.Printf("    - %s\n", name)
	}
	return 0
}

func (a App) Import(path, target string, allowRunning bool) int {
	resolved := abs(expandHome(path))
	raw, err := os.ReadFile(resolved)
	if err != nil {
		fmt.Printf("  [error] file not found: %s\n", resolved)
		return 1
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		fmt.Printf("  [error] cannot parse import file: %v\n", err)
		return 1
	}
	targetPayloads := asMap(payload["targets"])
	if len(targetPayloads) == 0 {
		fmt.Println("  [error] invalid export format - missing 'targets' key")
		return 1
	}
	if !allowRunning && HeliumRunning() {
		fmt.Println("ERROR: Helium is running on this device.")
		fmt.Println("Import writes directly to your Helium profile; close Helium first.")
		fmt.Println("(Or, for testing: --allow-helium-running)")
		return 4
	}
	backup := filepath.Join(a.LogsDir, "preImport."+nowStamp())
	_ = os.MkdirAll(backup, 0755)
	applied := 0
	for _, t := range a.targetByName(target) {
		targetPayload := asMap(targetPayloads[t.Name()])
		if len(targetPayload) == 0 {
			continue
		}
		fmt.Printf("  apply %s...\n", t.Name())
		if err := t.Apply(a.Profile, targetPayload["data"], backup); err != nil {
			fmt.Printf("  apply %s failed: %v\n", t.Name(), err)
			return 5
		}
		fmt.Printf("  %s: applied\n", t.Name())
		applied++
	}
	if applied == 0 {
		fmt.Println("  no matching targets found in import file")
		return 1
	}
	fmt.Printf("  backup: %s\n", rel(a.RepoRoot, backup))
	fmt.Printf("  done. %d target(s) applied. Launch Helium to see the imported state.\n", applied)
	return 0
}
