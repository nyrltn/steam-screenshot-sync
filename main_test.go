package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadUserDir(t *testing.T) {
	home := "/home/u"
	path := filepath.Join(t.TempDir(), "user-dirs.dirs")
	os.WriteFile(path, []byte(`# comment
XDG_DESKTOP_DIR="$HOME/Desktop"
XDG_PICTURES_DIR="$HOME/Bilder"
`), 0o644)
	if got := readUserDir(path, "XDG_PICTURES_DIR", home); got != "/home/u/Bilder" {
		t.Errorf("got %q", got)
	}

	os.WriteFile(path, []byte(`XDG_PICTURES_DIR="$HOME/"`), 0o644)
	if got := readUserDir(path, "XDG_PICTURES_DIR", home); got != "" {
		t.Errorf("disabled dir should be ignored, got %q", got)
	}
	if got := readUserDir("/nonexistent", "XDG_PICTURES_DIR", home); got != "" {
		t.Errorf("missing file should yield empty, got %q", got)
	}
}
