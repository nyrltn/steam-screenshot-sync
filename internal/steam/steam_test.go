package steam

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func manifest(id, name string) string {
	return "\"AppState\"\n{\n\t\"appid\"\t\t\"" + id + "\"\n\t\"name\"\t\t\"" + name + "\"\n}\n"
}

func fakeSteam(t *testing.T) (root, lib2 string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "Steam")
	lib2 = filepath.Join(base, "Library2")

	write(t, filepath.Join(root, "steamapps", "libraryfolders.vdf"), `"libraryfolders"
{
	"0" { "path" "`+root+`" }
	"1" { "path" "`+lib2+`" }
}`)
	write(t, filepath.Join(root, "steamapps", "appmanifest_100.acf"), manifest("100", "Game: One"))
	write(t, filepath.Join(lib2, "steamapps", "appmanifest_200.acf"), manifest("200", "Game/Two"))
	write(t, filepath.Join(lib2, "steamapps", "appmanifest_300.acf"), "\"AppState\" {")

	write(t, filepath.Join(root, "userdata", "1", "760", "remote", "100", "screenshots", "a.jpg"), "a")
	write(t, filepath.Join(root, "userdata", "2", "760", "remote", "200", "screenshots", "b.jpg"), "b")
	// A remote dir without screenshots/ must be ignored.
	if err := os.MkdirAll(filepath.Join(root, "userdata", "1", "760", "remote", "999"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, lib2
}

func TestFindRoot(t *testing.T) {
	root, _ := fakeSteam(t)
	got, ok := FindRoot([]string{"", filepath.Join(root, "nope"), root})
	if !ok || got != root {
		t.Fatalf("FindRoot = %q, %v", got, ok)
	}
	if _, ok := FindRoot([]string{t.TempDir()}); ok {
		t.Error("dir without userdata should not count as a Steam root")
	}
}

func TestLibrariesAndNames(t *testing.T) {
	root, lib2 := fakeSteam(t)

	// A symlink to root listed as a library must be deduplicated.
	link := filepath.Join(filepath.Dir(root), "steamlink")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	libs, err := LibraryPaths(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(libs) != 2 {
		t.Fatalf("LibraryPaths = %v, want 2 entries", libs)
	}

	names, errs := AppNames([]string{root, lib2})
	want := map[string]string{"100": "Game: One", "200": "Game/Two"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("AppNames = %v, want %v", names, want)
	}
	if len(errs) != 1 {
		t.Errorf("want 1 error for the broken manifest, got %v", errs)
	}
}

func TestScreenshotDirs(t *testing.T) {
	root, _ := fakeSteam(t)
	dirs, err := ScreenshotDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 2 || dirs[0].AppID != "100" || dirs[1].AppID != "200" {
		t.Fatalf("ScreenshotDirs = %+v", dirs)
	}
}

func shortcutsVDF(appid uint32, name string) []byte {
	b := []byte{0x00}
	b = append(b, "shortcuts\x00"...)
	b = append(b, 0x00)
	b = append(b, "0\x00"...)
	b = append(b, 0x02)
	b = append(b, "appid\x00"...)
	b = binary.LittleEndian.AppendUint32(b, appid)
	b = append(b, 0x01)
	b = append(b, "AppName\x00"...)
	b = append(b, name...)
	b = append(b, 0, 0x08, 0x08, 0x08)
	return b
}

func TestShortcutGameID(t *testing.T) {
	if got := ShortcutGameID(3487208137); got != "14977444902793641984" {
		t.Errorf("ShortcutGameID = %s", got)
	}
}

func TestShortcutShortID(t *testing.T) {
	if got := ShortcutShortID(2405898600); got != "6756712" {
		t.Errorf("ShortcutShortID = %s", got)
	}
}

func TestShortcutNames(t *testing.T) {
	root, _ := fakeSteam(t)
	write(t, filepath.Join(root, "userdata", "1", "config", "shortcuts.vdf"),
		string(shortcutsVDF(3487208137, "Arknights")))
	write(t, filepath.Join(root, "userdata", "2", "config", "shortcuts.vdf"), "\x00broken")
	write(t, filepath.Join(root, "userdata", "1", "760", "remote", "14977444902793641984", "screenshots", "c.jpg"), "c")

	names, errs := ShortcutNames(root)
	if !reflect.DeepEqual(names, map[string]string{
		"14977444902793641984": "Arknights",
		"14324425":             "Arknights",
	}) {
		t.Errorf("ShortcutNames = %v", names)
	}
	if len(errs) != 1 {
		t.Errorf("want 1 error for the broken file, got %v", errs)
	}

	dirs, _ := ScreenshotDirs(root)
	found := false
	for _, d := range dirs {
		if names[d.AppID] == "Arknights" {
			found = true
		}
	}
	if !found {
		t.Error("shortcut screenshot folder does not match the name key")
	}
}
