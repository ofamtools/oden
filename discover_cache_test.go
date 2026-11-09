package oden

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// Tests here swap package seams (scanner, cacheDir) and the env, so they
// run one at a time.

type cacheEnv struct {
	bin, cache string
	scans      *atomic.Int32
}

func newCacheEnv(t *testing.T) cacheEnv {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	cache := filepath.Join(root, "cache")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	var n atomic.Int32
	oldScan, oldDir := scanner, cacheDir
	scanner = func(b []string, p string, f FS) ([]External, []External) {
		n.Add(1)
		return scanDiscoverAll(b, p, f)
	}
	cacheDir = func() string { return cache }
	t.Cleanup(func() { scanner, cacheDir = oldScan, oldDir })
	t.Setenv(EnvDiscoverCache, "")
	return cacheEnv{bin: bin, cache: cache, scans: &n}
}

func (c cacheEnv) add(t *testing.T, name string) {
	t.Helper()
	p := filepath.Join(c.bin, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Directory times can tie on a coarse clock; push it forward.
	future := time.Now().Add(time.Duration(len(name)+int(c.scans.Load())+1) * time.Second)
	os.Chtimes(c.bin, future, future)
}

func (c cacheEnv) names(t *testing.T) []string {
	t.Helper()
	found, _ := DiscoverAll([]string{"tb"}, c.bin, OS)
	var out []string
	for _, e := range found {
		out = append(out, e.Name)
	}
	return out
}

// forgetMemo simulates a new process: the memo is empty, the disk stays.
func forgetMemo() {
	memoMu.Lock()
	memo = map[string]cacheEntry{}
	memoMu.Unlock()
}

func TestDiscoverMemoScansOnceForManyCalls(t *testing.T) {
	c := newCacheEnv(t)
	forgetMemo()
	c.add(t, "tb-one")
	for i := 0; i < 20; i++ {
		if got := c.names(t); len(got) != 1 || got[0] != "one" {
			t.Fatalf("call %d: %v", i, got)
		}
	}
	if n := c.scans.Load(); n != 1 {
		t.Fatalf("scans = %d, want 1", n)
	}
}

func TestDiscoverDiskCacheHitAcrossProcesses(t *testing.T) {
	c := newCacheEnv(t)
	forgetMemo()
	c.add(t, "tb-one")
	c.names(t)
	if _, err := os.Stat(filepath.Join(c.cache, "discover.json")); err != nil {
		t.Fatal("cache file not written:", err)
	}
	forgetMemo()
	if got := c.names(t); len(got) != 1 {
		t.Fatalf("%v", got)
	}
	if n := c.scans.Load(); n != 1 {
		t.Fatalf("scans = %d, want 1 (second process should hit the disk cache)", n)
	}
}

func TestDiscoverInvalidatesOnDirChange(t *testing.T) {
	c := newCacheEnv(t)
	forgetMemo()
	c.add(t, "tb-one")
	c.names(t)
	c.add(t, "tb-two")
	if got := c.names(t); len(got) != 2 {
		t.Fatalf("memo missed an added binary: %v", got)
	}
	forgetMemo()
	os.Remove(filepath.Join(c.bin, "tb-one"))
	future := time.Now().Add(time.Hour)
	os.Chtimes(c.bin, future, future)
	if got := c.names(t); len(got) != 1 || got[0] != "two" {
		t.Fatalf("disk cache missed a removed binary: %v", got)
	}
	if n := c.scans.Load(); n != 3 {
		t.Fatalf("scans = %d, want 3", n)
	}
}

func TestDiscoverInvalidatesOnChmod(t *testing.T) {
	c := newCacheEnv(t)
	forgetMemo()
	c.add(t, "tb-one")
	c.names(t)
	forgetMemo()
	os.Chmod(filepath.Join(c.bin, "tb-one"), 0o644)
	if got := c.names(t); len(got) != 0 {
		t.Fatalf("stale executable served: %v", got)
	}
}

func TestDiscoverInvalidatesOnPathChange(t *testing.T) {
	c := newCacheEnv(t)
	forgetMemo()
	c.add(t, "tb-one")
	c.names(t)
	other := t.TempDir()
	os.WriteFile(filepath.Join(other, "tb-zed"), []byte("x"), 0o755)
	found, _ := DiscoverAll([]string{"tb"}, other+string(os.PathListSeparator)+c.bin, OS)
	if len(found) != 2 {
		t.Fatalf("%v", found)
	}
	if n := c.scans.Load(); n != 2 {
		t.Fatalf("scans = %d, want 2", n)
	}
	// Both PATH keys now live in the cache file.
	forgetMemo()
	c.names(t)
	DiscoverAll([]string{"tb"}, other+string(os.PathListSeparator)+c.bin, OS)
	if n := c.scans.Load(); n != 2 {
		t.Fatalf("scans = %d after reload, want 2", n)
	}
}

func TestDiscoverCorruptCacheRescans(t *testing.T) {
	c := newCacheEnv(t)
	forgetMemo()
	c.add(t, "tb-one")
	os.MkdirAll(c.cache, 0o755)
	os.WriteFile(filepath.Join(c.cache, "discover.json"), []byte("{not json"), 0o644)
	if got := c.names(t); len(got) != 1 {
		t.Fatalf("%v", got)
	}
	forgetMemo()
	c.names(t) // rewritten file is good now
	if n := c.scans.Load(); n != 1 {
		t.Fatalf("scans = %d, want 1", n)
	}
}

func TestDiscoverOptOutWritesNoDiskCache(t *testing.T) {
	c := newCacheEnv(t)
	forgetMemo()
	t.Setenv(EnvDiscoverCache, "off")
	c.add(t, "tb-one")
	c.names(t)
	if _, err := os.Stat(c.cache); err == nil {
		t.Fatal("cache dir created with the disk cache off")
	}
	forgetMemo()
	c.names(t)
	if n := c.scans.Load(); n != 2 {
		t.Fatalf("scans = %d, want 2 (no disk cache)", n)
	}
}

type wrapFS struct{ FS }

func TestDiscoverFakeFSIsNeverCached(t *testing.T) {
	c := newCacheEnv(t)
	forgetMemo()
	c.add(t, "tb-one")
	for i := 0; i < 3; i++ {
		DiscoverAll([]string{"tb"}, c.bin, wrapFS{OS})
	}
	if n := c.scans.Load(); n != 0 {
		t.Fatalf("fake FS went through the cache: %d", n)
	}
}

type keyedFS struct {
	FS
	key string
}

func (k keyedFS) DiscoverCacheKey() string { return k.key }

func TestDiscoverCachedFSKeysSeparateEntries(t *testing.T) {
	c := newCacheEnv(t)
	forgetMemo()
	c.add(t, "tb-one")
	bins := []string{"tb"}
	DiscoverAll(bins, c.bin, keyedFS{OS, "a"})
	DiscoverAll(bins, c.bin, keyedFS{OS, "a"})
	DiscoverAll(bins, c.bin, keyedFS{OS, "b"})
	if n := c.scans.Load(); n != 2 {
		t.Fatalf("scans = %d, want 2 (one per key)", n)
	}
	DiscoverAll(bins, c.bin, keyedFS{OS, ""})
	DiscoverAll(bins, c.bin, keyedFS{OS, ""})
	if n := c.scans.Load(); n != 2 {
		t.Fatalf("empty key must bypass the cache: %d", n)
	}
}
