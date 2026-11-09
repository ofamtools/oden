package oden

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FS is the file system discovery reads: directory listings and Stat
// (which follows symlinks). OS is the real one; tests pass a fake.
type FS interface {
	ReadDir(dir string) ([]fs.DirEntry, error)
	Stat(path string) (fs.FileInfo, error)
}

// CachedFS is an FS that reads the real disk, perhaps leaving some names
// out, and so lets discovery remember its results. DiscoverCacheKey names
// what it leaves out (any stable text, equal for equal filtering); a
// different key is a different cache entry. An empty key opts out.
type CachedFS interface {
	FS
	DiscoverCacheKey() string
}

type osFS struct{}

func (osFS) DiscoverCacheKey() string { return "os" }

func (osFS) ReadDir(dir string) ([]fs.DirEntry, error) { return os.ReadDir(dir) }
func (osFS) Stat(path string) (fs.FileInfo, error)     { return os.Stat(path) }

// OS is the operating system's file system.
var OS FS = osFS{}

// PathDirs splits a PATH list in order: an empty entry is the current
// directory, and a repeated directory is kept once, at its first place.
func PathDirs(path string) []string {
	var dirs []string
	seen := map[string]bool{}
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			dir = "."
		}
		dir = filepath.Clean(dir)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	return dirs
}

// Executable reports whether path is a regular file (or a link to one)
// with an executable bit.
func Executable(fsys FS, path string) bool {
	info, err := fsys.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// Discover scans the PATH directories for executables named <bin>-<sub>.
// The first one on PATH for each sub is found; later ones are shadowed.
// Unreadable directories, directories and non-executables are skipped.
// Both lists are in PATH order, then by name within a directory.
func Discover(bin, path string, fsys FS) (found, shadowed []External) {
	prefix := bin + "-"
	taken := map[string]bool{}
	for _, dir := range PathDirs(path) {
		entries, err := fsys.ReadDir(dir)
		if err != nil {
			continue
		}
		var names []string
		for _, e := range entries {
			if name := e.Name(); strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			full := filepath.Join(dir, name)
			if !Executable(fsys, full) {
				continue
			}
			ext := External{Name: strings.TrimPrefix(name, prefix), Path: full, Prefix: bin}
			if taken[ext.Name] {
				shadowed = append(shadowed, ext)
				continue
			}
			taken[ext.Name] = true
			found = append(found, ext)
		}
	}
	return found, shadowed
}

// DiscoverAll discovers each prefix in turn: every <bin>-<sub> on PATH
// first, then every <extra>-<sub> per extra prefix, in the given order.
// A sub already found under an earlier prefix shadows the later ones.
//
// With the real file system (OS), or an FS that says it reads it
// (CachedFS), the result is remembered: in memory for the rest of the
// process, and on disk across processes (discover_cache.go), each checked
// against the modification time of every PATH directory, so a binary added
// to or removed from one is seen at once. Any other FS (a fake) is never
// cached.
func DiscoverAll(bins []string, path string, fsys FS) (found, shadowed []External) {
	if key, ok := cacheKeyOf(fsys); ok {
		return cachedDiscoverAll(bins, path, key, fsys)
	}
	return scanDiscoverAll(bins, path, fsys)
}

// scanDiscoverAll is DiscoverAll without any cache: it reads the disk.
func scanDiscoverAll(bins []string, path string, fsys FS) (found, shadowed []External) {
	taken := map[string]bool{}
	for _, bin := range bins {
		f, s := Discover(bin, path, fsys)
		for _, ext := range f {
			if taken[ext.Name] {
				shadowed = append(shadowed, ext)
				continue
			}
			taken[ext.Name] = true
			found = append(found, ext)
		}
		shadowed = append(shadowed, s...)
	}
	return found, shadowed
}
