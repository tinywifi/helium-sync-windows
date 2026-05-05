package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tinywifi/helium-sync-windows/internal/heliumsync"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	global := flag.NewFlagSet("helium-sync", flag.ContinueOnError)
	global.SetOutput(os.Stderr)
	profile := global.String("profile", heliumsync.DefaultProfile(), "Helium profile directory")
	repo := global.String("repo", "", "data repo path")
	if err := global.Parse(args); err != nil {
		return 2
	}
	rest := global.Args()
	if len(rest) == 0 {
		usage()
		return 2
	}
	app := heliumsync.New(*repo, *profile)
	app.MaybeShowUpdateBanner()
	switch rest[0] {
	case "setup":
		fs := flag.NewFlagSet("setup", flag.ContinueOnError)
		force := fs.Bool("force", false, "overwrite existing config")
		yes := fs.Bool("yes", false, "skip confirmation prompts")
		fs.BoolVar(yes, "y", false, "skip confirmation prompts")
		_ = fs.Parse(rest[1:])
		return app.Setup(*force, *yes)
	case "push":
		fs := flag.NewFlagSet("push", flag.ContinueOnError)
		target := fs.String("target", "", "sync only this target")
		strict := fs.Bool("strict", false, "fail on validation warnings")
		dryRun := fs.Bool("dry-run", false, "show changes without committing")
		_ = fs.Parse(rest[1:])
		return app.Push(*target, *strict, *dryRun)
	case "pull":
		fs := flag.NewFlagSet("pull", flag.ContinueOnError)
		target := fs.String("target", "", "sync only this target")
		allow := fs.Bool("allow-helium-running", false, "bypass running-browser guard")
		dryRun := fs.Bool("dry-run", false, "show changes without writing")
		_ = fs.Parse(rest[1:])
		return app.Pull(*target, *allow, *dryRun)
	case "status":
		fs := flag.NewFlagSet("status", flag.ContinueOnError)
		target := fs.String("target", "", "show only this target")
		_ = fs.Parse(rest[1:])
		return app.Status(*target)
	case "diff":
		fs := flag.NewFlagSet("diff", flag.ContinueOnError)
		target := fs.String("target", "", "diff only this target")
		_ = fs.Parse(rest[1:])
		return app.Diff(*target)
	case "doctor":
		return app.Doctor()
	case "version":
		return app.Version()
	case "init":
		fs := flag.NewFlagSet("init", flag.ContinueOnError)
		force := fs.Bool("force", false, "overwrite canonical state")
		target := fs.String("target", "", "init only this target")
		_ = fs.Parse(rest[1:])
		return app.Init(*target, *force, false, false)
	case "adopt":
		fs := flag.NewFlagSet("adopt", flag.ContinueOnError)
		yes := fs.Bool("yes", false, "skip confirmation")
		fs.BoolVar(yes, "y", false, "skip confirmation")
		allow := fs.Bool("allow-helium-running", false, "bypass running-browser guard")
		_ = fs.Parse(rest[1:])
		return app.Adopt(*yes, *allow)
	case "log":
		fs := flag.NewFlagSet("log", flag.ContinueOnError)
		n := fs.Int("n", 10, "number of commits")
		_ = fs.Parse(rest[1:])
		return app.Log(*n)
	case "gc":
		fs := flag.NewFlagSet("gc", flag.ContinueOnError)
		keep := fs.Int("keep-days", 30, "days to keep")
		dryRun := fs.Bool("dry-run", false, "show deletions")
		_ = fs.Parse(rest[1:])
		return app.GC(*keep, *dryRun)
	case "export":
		fs := flag.NewFlagSet("export", flag.ContinueOnError)
		output := fs.String("output", "", "output file")
		target := fs.String("target", "", "export only this target")
		_ = fs.Parse(rest[1:])
		return app.Export(*output, *target)
	case "import":
		fs := flag.NewFlagSet("import", flag.ContinueOnError)
		target := fs.String("target", "", "import only this target")
		allow := fs.Bool("allow-helium-running", false, "bypass running-browser guard")
		_ = fs.Parse(rest[1:])
		if fs.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "import requires a file")
			return 2
		}
		return app.Import(fs.Arg(0), *target, *allow)
	case "restore":
		fs := flag.NewFlagSet("restore", flag.ContinueOnError)
		allow := fs.Bool("allow-helium-running", false, "bypass running-browser guard")
		_ = fs.Parse(rest[1:])
		return app.Restore(*allow)
	case "resolve":
		fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
		target := fs.String("target", "bookmarks", "target to resolve")
		theirs := fs.String("theirs", "", "canonical JSON file")
		_ = fs.Parse(rest[1:])
		return app.Resolve(*target, *theirs)
	case "completion":
		fs := flag.NewFlagSet("completion", flag.ContinueOnError)
		shell := fs.String("shell", "", "powershell or cmd")
		_ = fs.Parse(rest[1:])
		return app.Completion(*shell)
	default:
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: helium-sync [--profile PATH] [--repo PATH] <setup|push|pull|status|diff|doctor|version|init|adopt|log|gc|export|import|restore|resolve|completion>")
}
