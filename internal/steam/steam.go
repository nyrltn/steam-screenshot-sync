// Package steam finds a Steam installation's libraries, installed game
// names and per-game screenshot directories.
package steam

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"steam-screenshot-sync/internal/vdf"
)

func DefaultRoots(home string) []string {
	return []string{
		filepath.Join(home, ".local/share/Steam"),
		filepath.Join(home, ".steam/steam"),
		filepath.Join(home, ".steam/root"),
		filepath.Join(home, ".var/app/com.valvesoftware.Steam/.local/share/Steam"),
	}
}

// A Steam root is recognised by its userdata directory.
func FindRoot(candidates []string) (string, bool) {
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if fi, err := os.Stat(filepath.Join(c, "userdata")); err == nil && fi.IsDir() {
			return c, true
		}
	}
	return "", false
}

// Includes root itself, deduplicated after resolving symlinks. A missing
// libraryfolders.vdf is not an error.
func LibraryPaths(root string) ([]string, error) {
	paths := []string{root}
	n, err := vdf.ParseFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return dedupe(paths), err
	default:
		if lf, ok := n.Child("libraryfolders"); ok {
			for _, v := range lf {
				if entry, ok := v.(vdf.Node); ok {
					if p, ok := entry.Get("path"); ok && p != "" {
						paths = append(paths, p)
					}
				}
			}
		}
	}
	return dedupe(paths), nil
}

func dedupe(paths []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		key := filepath.Clean(p)
		if r, err := filepath.EvalSymlinks(key); err == nil {
			key = r
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// Unparseable manifests are reported in errs but do not stop the scan.
func AppNames(libraries []string) (names map[string]string, errs []error) {
	names = map[string]string{}
	for _, lib := range libraries {
		matches, _ := filepath.Glob(filepath.Join(lib, "steamapps", "appmanifest_*.acf"))
		for _, m := range matches {
			n, err := vdf.ParseFile(m)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			id, _ := n.Get("AppState", "appid")
			name, _ := n.Get("AppState", "name")
			if id == "" {
				id = strings.TrimSuffix(strings.TrimPrefix(filepath.Base(m), "appmanifest_"), ".acf")
			}
			if name != "" {
				names[id] = name
			}
		}
	}
	return names, errs
}

type ScreenshotDir struct {
	AppID string
	Path  string
}

func ScreenshotDirs(root string) ([]ScreenshotDir, error) {
	matches, err := filepath.Glob(filepath.Join(root, "userdata", "*", "760", "remote", "*", "screenshots"))
	if err != nil {
		return nil, err
	}
	var out []ScreenshotDir
	for _, m := range matches {
		if fi, err := os.Stat(m); err != nil || !fi.IsDir() {
			continue
		}
		out = append(out, ScreenshotDir{AppID: filepath.Base(filepath.Dir(m)), Path: m})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Steam stores a non-Steam shortcut's screenshots under this 64-bit ID,
// not under the 32-bit appid from shortcuts.vdf.
func ShortcutGameID(appid uint32) string {
	return strconv.FormatUint(uint64(appid)<<32|0x02000000, 10)
}

// Names of non-Steam games added to the library, keyed by screenshot
// folder name. Unparseable files are reported in errs.
func ShortcutNames(root string) (names map[string]string, errs []error) {
	names = map[string]string{}
	files, _ := filepath.Glob(filepath.Join(root, "userdata", "*", "config", "shortcuts.vdf"))
	for _, f := range files {
		n, err := vdf.ParseBinaryFile(f)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		list, _ := n.Child("shortcuts")
		for _, v := range list {
			entry, ok := v.(vdf.Node)
			if !ok {
				continue
			}
			name, _ := entry.Get("appname")
			id, _ := entry.Get("appid")
			appid, err := strconv.ParseUint(id, 10, 32)
			if name == "" || err != nil {
				continue
			}
			names[ShortcutGameID(uint32(appid))] = name
		}
	}
	return names, errs
}
