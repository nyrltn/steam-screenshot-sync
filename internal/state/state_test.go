package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "names.json")
	cache, err := LoadNames(path)
	if err != nil || len(cache) != 0 {
		t.Fatalf("missing file: %v %v", cache, err)
	}
	cache["1"] = "Old"
	cache["2"] = "Uninstalled"
	if !MergeNames(cache, map[string]string{"1": "New"}) {
		t.Error("MergeNames should report a change")
	}
	if MergeNames(cache, map[string]string{"1": "New"}) {
		t.Error("MergeNames should report no change")
	}
	if err := SaveNames(path, cache); err != nil {
		t.Fatal(err)
	}
	got, err := LoadNames(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]string{"1": "New", "2": "Uninstalled"}) {
		t.Errorf("round trip = %v", got)
	}
}

func TestSynced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "synced")
	s, err := LoadSynced(path)
	if err != nil || s.Len() != 0 {
		t.Fatalf("missing file: %v", err)
	}

	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Save without changes should not create the file")
	}

	s.Add("b/2.jpg")
	s.Add("a/1.jpg")
	s.Add("a/1.jpg")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "a/1.jpg\nb/2.jpg\n" {
		t.Errorf("file = %q", b)
	}

	s2, err := LoadSynced(path)
	if err != nil {
		t.Fatal(err)
	}
	if !s2.Has("a/1.jpg") || !s2.Has("b/2.jpg") || s2.Has("c") {
		t.Error("round trip lost keys")
	}
	s2.Add("a/1.jpg")
	os.Remove(path)
	if err := s2.Save(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("adding a known key should not mark the set dirty")
	}
}
