// Package state keeps what the tool must remember between runs and cannot
// rebuild: names of games that are no longer installed, and which
// screenshots were already synced, so ones the user deleted stay deleted.
package state

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A missing file yields an empty map.
func LoadNames(path string) (map[string]string, error) {
	names := map[string]string{}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return names, err
	}
	if err := json.Unmarshal(b, &names); err != nil {
		return map[string]string{}, fmt.Errorf("%s: %w", path, err)
	}
	return names, nil
}

func SaveNames(path string, names map[string]string) error {
	b, err := json.MarshalIndent(names, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(b, '\n'))
}

// Reports whether cache changed. Fresh names win over cached ones.
func MergeNames(cache, fresh map[string]string) bool {
	changed := false
	for id, name := range fresh {
		if cache[id] != name {
			cache[id] = name
			changed = true
		}
	}
	return changed
}

type Synced struct {
	keys  map[string]bool
	dirty bool
}

// A missing file yields an empty set.
func LoadSynced(path string) (*Synced, error) {
	s := &Synced{keys: map[string]bool{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			s.keys[line] = true
		}
	}
	return s, sc.Err()
}

func (s *Synced) Has(key string) bool { return s.keys[key] }

func (s *Synced) Add(key string) {
	if !s.keys[key] {
		s.keys[key] = true
		s.dirty = true
	}
}

func (s *Synced) Len() int { return len(s.keys) }

// Does nothing unless keys were added since loading.
func (s *Synced) Save(path string) error {
	if !s.dirty {
		return nil
	}
	lines := make([]string, 0, len(s.keys))
	for k := range s.keys {
		lines = append(lines, k)
	}
	sort.Strings(lines)
	if err := writeAtomic(path, []byte(strings.Join(lines, "\n")+"\n")); err != nil {
		return err
	}
	s.dirty = false
	return nil
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
