// Command steam-screenshot-sync hard-links Steam screenshots into
// ~/Pictures/Steam/<Game Name>/ so they survive deletion of Steam's copies.
// It runs once and exits; a systemd user timer runs it periodically.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"steam-screenshot-sync/internal/linker"
	"steam-screenshot-sync/internal/state"
	"steam-screenshot-sync/internal/steam"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("error: %v", err)
		return 1
	}

	steamDir := flag.String("steam-dir", "", "Steam root (default: $STEAM_DIR, then common locations)")
	dest := flag.String("dest", filepath.Join(picturesDir(home), "Steam"), "destination root")
	stateDir := flag.String("state-dir", filepath.Join(stateHome(home), "steam-screenshot-sync"), "where game names and the synced-files record are kept")
	dryRun := flag.Bool("dry-run", false, "print what would be done without changing anything")
	verbose := flag.Bool("v", false, "log every file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	log.SetFlags(0) // journald adds timestamps
	if *showVersion {
		fmt.Println(version)
		return 0
	}

	candidates := []string{*steamDir}
	if *steamDir == "" {
		candidates = append([]string{os.Getenv("STEAM_DIR")}, steam.DefaultRoots(home)...)
	}
	root, ok := steam.FindRoot(candidates)
	if !ok {
		log.Printf("no Steam installation found, nothing to do")
		return 0
	}

	failed := false
	warn := func(err error) {
		log.Printf("warning: %v", err)
	}

	libs, err := steam.LibraryPaths(root)
	if err != nil {
		warn(err)
	}
	fresh, errs := steam.AppNames(libs)
	shortcuts, scErrs := steam.ShortcutNames(root)
	for _, e := range append(errs, scErrs...) {
		warn(e)
	}
	for id, name := range shortcuts {
		if _, ok := fresh[id]; !ok {
			fresh[id] = name
		}
	}

	namesPath := filepath.Join(*stateDir, "names.json")
	names, err := state.LoadNames(namesPath)
	if err != nil {
		warn(err) // a corrupt file is rebuilt from manifests and shortcuts
	}
	if state.MergeNames(names, fresh) && !*dryRun {
		if err := state.SaveNames(namesPath, names); err != nil {
			warn(err)
		}
	}

	syncedPath := filepath.Join(*stateDir, "synced")
	synced, err := state.LoadSynced(syncedPath)
	if err != nil {
		// Carrying on without the record would bring back deleted screenshots.
		log.Printf("error: %v", err)
		return 1
	}

	dirs, err := steam.ScreenshotDirs(root)
	if err != nil {
		log.Printf("error: %v", err)
		return 1
	}

	opt := linker.Options{DryRun: *dryRun, Synced: synced}
	if *verbose {
		opt.Logf = log.Printf
	}
	var total linker.Stats
	for _, d := range dirs {
		name := names[d.AppID]
		if name == "" {
			name = d.AppID
		}
		opt.KeyPrefix = d.AppID + "/"
		st := linker.Sync(d.Path, filepath.Join(*dest, linker.SanitizeName(name)), opt)
		for _, e := range st.Errors {
			log.Printf("error: %v", e)
			failed = true
		}
		total.Add(st)
	}

	if !*dryRun {
		if err := synced.Save(syncedPath); err != nil {
			log.Printf("error: %v", err)
			failed = true
		}
	}

	prefix := ""
	if *dryRun {
		prefix = "dry run: "
	}
	log.Printf("%s%d screenshot folders from %s into %s: %s", prefix, len(dirs), root, *dest, total)
	if failed {
		return 1
	}
	return 0
}

func picturesDir(home string) string {
	if p := os.Getenv("XDG_PICTURES_DIR"); p != "" {
		return p
	}
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	if p := readUserDir(filepath.Join(cfg, "user-dirs.dirs"), "XDG_PICTURES_DIR", home); p != "" {
		return p
	}
	return filepath.Join(home, "Pictures")
}

// Per the xdg-user-dirs format, $HOME is the only expansion allowed.
func readUserDir(path, key, home string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(line, "#") || strings.TrimSpace(k) != key {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), `"`)
		v = strings.Replace(v, "$HOME", home, 1)
		if !filepath.IsAbs(v) || filepath.Clean(v) == filepath.Clean(home) {
			// A pictures dir equal to $HOME means "disabled" in xdg-user-dirs.
			return ""
		}
		return v
	}
	return ""
}

func stateHome(home string) string {
	if s := os.Getenv("XDG_STATE_HOME"); s != "" {
		return s
	}
	return filepath.Join(home, ".local", "state")
}
