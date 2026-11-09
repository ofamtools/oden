package oden

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// EnvDiscoverCache turns the on-disk discovery cache off when set to off,
// 0, false or no. The in-process memo stays (it is checked the same way).
const EnvDiscoverCache = "ODEN_DISCOVER_CACHE"

// discoverCacheMax bounds the entries kept in the cache file (one per
// distinct bins+PATH pair; a hook or a script with its own PATH adds one).
const discoverCacheMax = 8

// cacheEntry is one remembered discovery: the PATH directories' stamps at
// scan time and what the scan found.
type cacheEntry struct {
	Key      string     `json:"key"`
	Stamps   []string   `json:"stamps"`
	Found    []External `json:"found"`
	Shadowed []External `json:"shadowed"`
}

type cacheFile struct {
	Version int          `json:"version"`
	Entries []cacheEntry `json:"entries"`
}

var (
	memoMu sync.Mutex
	memo   = map[string]cacheEntry{}

	// scanner and cacheDir are seams for tests.
	scanner  = scanDiscoverAll
	cacheDir = defaultCacheDir
)

// cacheKeyOf is the cache key part of fsys, or false when fsys is not the
// real disk.
func cacheKeyOf(fsys FS) (string, bool) {
	c, ok := fsys.(CachedFS)
	if !ok {
		return "", false
	}
	k := c.DiscoverCacheKey()
	return k, k != ""
}

func cacheKey(fsKey string, bins []string, path string) string {
	// A relative PATH entry (an empty one is ".") means a different
	// directory in each working directory.
	cwd := ""
	for _, d := range PathDirs(path) {
		if !filepath.IsAbs(d) {
			cwd, _ = os.Getwd()
			break
		}
	}
	sum := sha256.Sum256([]byte(fsKey + "\x01" + strings.Join(bins, "\x00") + "\x01" + path + "\x01" + cwd))
	return hex.EncodeToString(sum[:])
}

// stamps is one string per PATH directory: its modification time (a
// directory's changes when an entry is added, removed or renamed in it),
// or "-" when it cannot be read.
func stamps(path string) []string {
	var out []string
	for _, dir := range PathDirs(path) {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			out = append(out, "-")
			continue
		}
		out = append(out, info.ModTime().UTC().Format("20060102T150405.000000000"))
	}
	return out
}

func sameStamps(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// stillExecutable re-checks the found files themselves, which catches a
// chmod the directory's time does not show.
func stillExecutable(e cacheEntry) bool {
	for _, x := range e.Found {
		if !Executable(OS, x.Path) {
			return false
		}
	}
	return true
}

func valid(e cacheEntry, stamp []string) bool {
	return sameStamps(e.Stamps, stamp) && stillExecutable(e)
}

func cachedDiscoverAll(bins []string, path, fsKey string, fsys FS) (found, shadowed []External) {
	key := cacheKey(fsKey, bins, path)
	stamp := stamps(path)

	memoMu.Lock()
	e, ok := memo[key]
	memoMu.Unlock()
	if ok && valid(e, stamp) {
		return append([]External(nil), e.Found...), append([]External(nil), e.Shadowed...)
	}

	diskOn := diskCacheEnabled()
	if diskOn {
		if e, ok := readDisk(key); ok && valid(e, stamp) {
			memoMu.Lock()
			memo[key] = e
			memoMu.Unlock()
			return append([]External(nil), e.Found...), append([]External(nil), e.Shadowed...)
		}
	}

	found, shadowed = scanner(bins, path, fsys)
	e = cacheEntry{Key: key, Stamps: stamp, Found: found, Shadowed: shadowed}
	memoMu.Lock()
	memo[key] = e
	memoMu.Unlock()
	if diskOn {
		writeDisk(e)
	}
	return found, shadowed
}

func diskCacheEnabled() bool {
	switch strings.ToLower(os.Getenv(EnvDiscoverCache)) {
	case "off", "0", "false", "no":
		return false
	}
	return cacheDir() != ""
}

func defaultCacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		var err error
		if base, err = os.UserCacheDir(); err != nil {
			return ""
		}
	}
	return filepath.Join(base, "oden")
}

func cachePath() string { return filepath.Join(cacheDir(), "discover.json") }

func loadFile() cacheFile {
	var f cacheFile
	data, err := os.ReadFile(cachePath())
	if err != nil || json.Unmarshal(data, &f) != nil || f.Version != 1 {
		return cacheFile{Version: 1}
	}
	return f
}

func readDisk(key string) (cacheEntry, bool) {
	for _, e := range loadFile().Entries {
		if e.Key == key {
			return e, true
		}
	}
	return cacheEntry{}, false
}

// writeDisk puts e first in the cache file, atomically (write a sibling,
// rename). A failure is ignored: the cache is only an accelerator.
func writeDisk(e cacheEntry) {
	f := loadFile()
	entries := []cacheEntry{e}
	for _, old := range f.Entries {
		if old.Key != e.Key && len(entries) < discoverCacheMax {
			entries = append(entries, old)
		}
	}
	f.Entries = entries
	data, err := json.Marshal(f)
	if err != nil {
		return
	}
	dir := cacheDir()
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, "discover-*.tmp")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), cachePath()) != nil {
		os.Remove(tmp.Name())
	}
}
