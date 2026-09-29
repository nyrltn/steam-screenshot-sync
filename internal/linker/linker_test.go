package linker

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
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

func setup(t *testing.T) (src, dst string) {
	t.Helper()
	base := t.TempDir()
	src = filepath.Join(base, "screenshots")
	dst = filepath.Join(base, "Pictures", "Steam", "Game")
	write(t, filepath.Join(src, "1.jpg"), "one")
	write(t, filepath.Join(src, "2.jpg"), "two")
	write(t, filepath.Join(src, "thumbnails", "1.jpg"), "thumb")
	return src, dst
}

func TestLinkIdempotentAndSurvivesDelete(t *testing.T) {
	src, dst := setup(t)

	st := Sync(src, dst, Options{})
	if st.Linked != 2 || len(st.Errors) != 0 {
		t.Fatalf("first run: %v %v", st, st.Errors)
	}
	a, _ := os.Stat(filepath.Join(src, "1.jpg"))
	b, _ := os.Stat(filepath.Join(dst, "1.jpg"))
	if !os.SameFile(a, b) {
		t.Error("destination is not a hard link of the source")
	}
	if _, err := os.Stat(filepath.Join(dst, "thumbnails")); !os.IsNotExist(err) {
		t.Error("thumbnails must not be mirrored")
	}

	st = Sync(src, dst, Options{})
	if st.Linked != 0 || st.Skipped != 2 {
		t.Fatalf("second run should skip everything: %v", st)
	}

	if err := os.RemoveAll(src); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "1.jpg"))
	if err != nil || string(got) != "one" {
		t.Fatalf("link did not survive deleting the source: %q %v", got, err)
	}
}

func TestConflictIsNotOverwritten(t *testing.T) {
	src, dst := setup(t)
	write(t, filepath.Join(dst, "1.jpg"), "user edited this")

	st := Sync(src, dst, Options{})
	if st.Conflicts != 1 || st.Linked != 1 {
		t.Fatalf("stats: %v", st)
	}
	got, _ := os.ReadFile(filepath.Join(dst, "1.jpg"))
	if string(got) != "user edited this" {
		t.Error("existing file was overwritten")
	}
}

func TestDryRunCreatesNothing(t *testing.T) {
	src, dst := setup(t)
	st := Sync(src, dst, Options{DryRun: true})
	if st.Linked != 2 {
		t.Fatalf("dry run stats: %v", st)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Error("dry run created the destination")
	}
}

func TestEmptySourceCreatesNoDir(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "screenshots")
	write(t, filepath.Join(src, "thumbnails", "x.jpg"), "x")
	dst := filepath.Join(base, "out", "Game")
	Sync(src, dst, Options{})
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Error("game folder created although there were no screenshots")
	}
}

func TestCrossDeviceFallsBackToCopy(t *testing.T) {
	src, dst := setup(t)
	linkFn = func(string, string) error {
		return &os.LinkError{Op: "link", Err: syscall.EXDEV}
	}
	t.Cleanup(func() { linkFn = os.Link; now = time.Now })

	// Make 2.jpg look freshly written: it must wait for a later run.
	fresh := time.Now()
	os.Chtimes(filepath.Join(src, "2.jpg"), fresh, fresh)
	os.Chtimes(filepath.Join(src, "1.jpg"), fresh.Add(-time.Hour), fresh.Add(-time.Hour))
	now = func() time.Time { return fresh }

	st := Sync(src, dst, Options{})
	if st.Copied != 1 || st.Pending != 1 || len(st.Errors) != 0 {
		t.Fatalf("stats: %v %v", st, st.Errors)
	}
	a, _ := os.Stat(filepath.Join(src, "1.jpg"))
	b, _ := os.Stat(filepath.Join(dst, "1.jpg"))
	if os.SameFile(a, b) {
		t.Error("expected a copy, got a link")
	}
	if !a.ModTime().Equal(b.ModTime()) {
		t.Error("copy did not keep the modification time")
	}

	now = func() time.Time { return fresh.Add(time.Minute) }
	st = Sync(src, dst, Options{})
	if st.Copied != 1 || st.Skipped != 1 {
		t.Fatalf("second run: %v", st)
	}
	leftovers, _ := filepath.Glob(filepath.Join(dst, ".sss-*"))
	if len(leftovers) != 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
}

func TestSanitizeName(t *testing.T) {
	for in, want := range map[string]string{
		"Master Detective Archives: RAIN CODE Plus": "Master Detective Archives: RAIN CODE Plus",
		"AC/DC Live":  "AC_DC Live",
		" ..hidden. ": "hidden",
		"..":          "_",
		"":            "_",
	} {
		if got := SanitizeName(in); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

type memSet map[string]bool

func (m memSet) Has(k string) bool { return m[k] }
func (m memSet) Add(k string)      { m[k] = true }

func TestDeletedStaysDeleted(t *testing.T) {
	src, dst := setup(t)
	set := memSet{}
	opt := Options{Synced: set, KeyPrefix: "100/"}

	if st := Sync(src, dst, opt); st.Linked != 2 {
		t.Fatalf("first run: %v", st)
	}
	if !set["100/1.jpg"] || !set["100/2.jpg"] || len(set) != 2 {
		t.Fatalf("recorded keys: %v", set)
	}

	os.Remove(filepath.Join(dst, "1.jpg"))
	st := Sync(src, dst, opt)
	if st.Removed != 1 || st.Linked != 0 || st.Skipped != 1 {
		t.Fatalf("after delete: %v", st)
	}
	if _, err := os.Stat(filepath.Join(dst, "1.jpg")); !os.IsNotExist(err) {
		t.Error("deleted screenshot came back")
	}

	// A wiped record brings it back.
	st = Sync(src, dst, Options{Synced: memSet{}, KeyPrefix: "100/"})
	if st.Linked != 1 {
		t.Fatalf("with fresh record: %v", st)
	}
}

func TestConflictAndPendingRecording(t *testing.T) {
	src, dst := setup(t)
	write(t, filepath.Join(dst, "1.jpg"), "user edited this")
	set := memSet{}
	Sync(src, dst, Options{Synced: set})
	if !set["1.jpg"] {
		t.Error("conflicting file should be recorded")
	}

	src2, dst2 := setup(t)
	linkFn = func(string, string) error { return &os.LinkError{Op: "link", Err: syscall.EXDEV} }
	t.Cleanup(func() { linkFn = os.Link })
	set2 := memSet{}
	if st := Sync(src2, dst2, Options{Synced: set2}); st.Pending != 2 {
		t.Fatalf("fresh files should be pending: %v", st)
	}
	if len(set2) != 0 {
		t.Errorf("pending files must not be recorded: %v", set2)
	}
}

func TestDryRunRecordsNothing(t *testing.T) {
	src, dst := setup(t)
	set := memSet{}
	Sync(src, dst, Options{DryRun: true, Synced: set})
	if len(set) != 0 {
		t.Errorf("dry run recorded %v", set)
	}
}
