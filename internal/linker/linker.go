// Package linker mirrors screenshot files into a destination directory
// as hard links, falling back to copies across filesystems. It never
// overwrites or deletes anything.
package linker

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Swapped out in tests.
var (
	linkFn = os.Link
	now    = time.Now
)

// Copies wait this long so a screenshot Steam is still writing isn't copied
// half-finished. Hard links need no delay because they share the inode.
const CopyMinAge = 30 * time.Second

type Stats struct {
	Linked    int
	Copied    int
	Skipped   int
	Conflicts int // a different file with the same name is already in dst
	Pending   int // too new to copy, picked up next run
	Removed   int // synced before, then deleted by the user; not brought back
	Errors    []error
}

func (s *Stats) Add(o Stats) {
	s.Linked += o.Linked
	s.Copied += o.Copied
	s.Skipped += o.Skipped
	s.Conflicts += o.Conflicts
	s.Pending += o.Pending
	s.Removed += o.Removed
	s.Errors = append(s.Errors, o.Errors...)
}

func (s Stats) String() string {
	return fmt.Sprintf("linked %d, copied %d, skipped %d, removed %d, conflicts %d, pending %d, errors %d",
		s.Linked, s.Copied, s.Skipped, s.Removed, s.Conflicts, s.Pending, len(s.Errors))
}

type SyncedSet interface {
	Has(key string) bool
	Add(key string)
}

type Options struct {
	DryRun bool
	Logf   func(format string, args ...any) // may be nil

	// Records synced files as KeyPrefix+filename. A recorded file whose
	// destination is gone was deleted by the user and is not synced again.
	// Nil disables this.
	Synced    SyncedSet
	KeyPrefix string
}

func (o Options) logf(format string, args ...any) {
	if o.Logf != nil {
		o.Logf(format, args...)
	}
}

func (o Options) seen(key string) bool {
	return o.Synced != nil && o.Synced.Has(key)
}

func (o Options) record(key string) {
	if o.Synced != nil && !o.DryRun {
		o.Synced.Add(key)
	}
}

func SanitizeName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == 0 {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" {
		return "_"
	}
	return name
}

// Subdirectories such as thumbnails/ are ignored. dst is created only when
// there is at least one file to place in it.
func Sync(src, dst string, opt Options) Stats {
	var st Stats
	entries, err := os.ReadDir(src)
	if err != nil {
		st.Errors = append(st.Errors, err)
		return st
	}
	madeDir := false
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		key := opt.KeyPrefix + e.Name()

		srcInfo, err := os.Stat(srcPath)
		if err != nil {
			st.Errors = append(st.Errors, err)
			continue
		}
		if dstInfo, err := os.Lstat(dstPath); err == nil {
			switch {
			case os.SameFile(srcInfo, dstInfo):
				st.Skipped++
			case dstInfo.Mode().IsRegular() && dstInfo.Size() == srcInfo.Size():
				st.Skipped++ // copied earlier across filesystems
			default:
				st.Conflicts++
				opt.logf("conflict, leaving alone: %s", dstPath)
			}
			opt.record(key)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			st.Errors = append(st.Errors, err)
			continue
		}

		if opt.seen(key) {
			st.Removed++
			continue
		}

		if opt.DryRun {
			opt.logf("would link %s -> %s", srcPath, dstPath)
			st.Linked++
			continue
		}
		if !madeDir {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				st.Errors = append(st.Errors, err)
				return st
			}
			madeDir = true
		}

		err = linkFn(srcPath, dstPath)
		switch {
		case err == nil:
			opt.logf("linked %s", dstPath)
			opt.record(key)
			st.Linked++
		case errors.Is(err, syscall.EXDEV) || errors.Is(err, syscall.EPERM):
			// EXDEV: different filesystem. EPERM: filesystem without hard
			// link support or fs.protected_hardlinks. Copy instead.
			if now().Sub(srcInfo.ModTime()) < CopyMinAge {
				st.Pending++
				continue
			}
			if err := copyFile(srcPath, dstPath, srcInfo); err != nil {
				st.Errors = append(st.Errors, err)
				continue
			}
			opt.logf("copied %s", dstPath)
			opt.record(key)
			st.Copied++
		default:
			st.Errors = append(st.Errors, err)
		}
	}
	return st
}

func copyFile(src, dst string, info os.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".sss-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_ = os.Chtimes(tmp.Name(), info.ModTime(), info.ModTime())
	// Rename replaces silently, so re-check that nothing appeared meanwhile.
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("%s appeared during copy, not replacing it", dst)
	}
	return os.Rename(tmp.Name(), dst)
}
