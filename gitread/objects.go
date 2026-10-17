package gitread

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Type names, as git spells them.
const (
	TypeCommit = "commit"
	TypeTree   = "tree"
	TypeBlob   = "blob"
	TypeTag    = "tag"
)

var kindNames = map[int]string{kindCommit: TypeCommit, kindTree: TypeTree, kindBlob: TypeBlob, kindTag: TypeTag}

// ErrMissing marks an object the repository does not hold. A partial clone leaves such objects on its promisor remote.
var ErrMissing = errors.New("gitread: the object is not in this repository")

// Cache budgets, in bytes. Git keeps delta bases to the same budget.
const (
	objectBudget = 64 << 20
	baseBudget   = 96 << 20
	treeBudget   = 64 << 20
)

type object struct {
	kind int
	data []byte
}

// odb is an object directory with its packs and alternates.
type odb struct {
	dir string

	mu       sync.Mutex
	packs    []*pack
	scanned  bool
	stamp    fileStamp
	alts     []*odb
	altsRead bool

	// objects keeps commits and tags, which callers read again.
	objects *lru[OID, object]
	// bases keeps resolved delta bases, which a delta chain reads again.
	bases *lru[baseKey, object]
	// trees keeps parsed trees, sorted for lookup.
	trees *lru[OID, []namedEntry]
}

func newODB(dir string) *odb {
	return &odb{dir: dir, objects: newLRU[OID, object](objectBudget), bases: newLRU[baseKey, object](baseBudget), trees: newLRU[OID, []namedEntry](treeBudget)}
}

// Object answers the type and content of an object.
func (r *Repo) Object(oid OID) (string, []byte, error) {
	obj, err := r.db.object(oid)
	if err != nil {
		return "", nil, err
	}
	return kindNames[obj.kind], obj.data, nil
}

// Has reports whether the object is in this repository, without reading it.
func (r *Repo) Has(oid OID) bool { return r.db.has(oid) }

func (db *odb) object(oid OID) (object, error) {
	if obj, ok := db.objects.get(oid); ok {
		return obj, nil
	}
	obj, err := db.find(oid, true)
	if err != nil {
		return object{}, err
	}
	if obj.kind == kindCommit || obj.kind == kindTag {
		db.objects.put(oid, obj, len(obj.data))
	}
	return obj, nil
}

// find reads oid from the packs, the loose store and the alternates. A miss
// scans the pack directory again, because a fetch can add a pack.
func (db *odb) find(oid OID, rescan bool) (object, error) {
	for _, p := range db.packList(false) {
		if off, ok := p.find(oid); ok {
			return p.read(db, off)
		}
	}
	if obj, err := db.loose(oid); !errors.Is(err, ErrMissing) {
		return obj, err
	}
	for _, alt := range db.alternates() {
		if obj, err := alt.find(oid, rescan); !errors.Is(err, ErrMissing) {
			return obj, err
		}
	}
	if rescan && db.packsChanged() {
		return db.find(oid, false)
	}
	return object{}, fmt.Errorf("%w: %s", ErrMissing, oid)
}

func (db *odb) has(oid OID) bool {
	if _, ok := db.objects.get(oid); ok {
		return true
	}
	for pass := 0; pass < 2; pass++ {
		for _, p := range db.packList(false) {
			if _, ok := p.find(oid); ok {
				return true
			}
		}
		hex := oid.String()
		if _, err := os.Stat(filepath.Join(db.dir, hex[:2], hex[2:])); err == nil {
			return true
		}
		for _, alt := range db.alternates() {
			if alt.has(oid) {
				return true
			}
		}
		if !db.packsChanged() {
			return false
		}
	}
	return false
}

// loose inflates objects/xx/yyyy.
func (db *odb) loose(oid OID) (object, error) {
	hex := oid.String()
	raw, err := os.ReadFile(filepath.Join(db.dir, hex[:2], hex[2:]))
	if err != nil {
		return object{}, ErrMissing
	}
	full, err := inflateAll(bytes.NewReader(raw))
	if err != nil {
		return object{}, fmt.Errorf("gitread: the loose object %s: %w", hex, err)
	}
	nul := bytes.IndexByte(full, 0)
	if nul < 0 {
		return object{}, fmt.Errorf("gitread: the loose object %s has no header", hex)
	}
	name, sizeText, ok := strings.Cut(string(full[:nul]), " ")
	size, perr := strconv.Atoi(sizeText)
	kind := kindOf(name)
	if !ok || perr != nil || kind == 0 || size != len(full)-nul-1 {
		return object{}, fmt.Errorf("gitread: %q is not an object header", full[:nul])
	}
	return object{kind: kind, data: full[nul+1:]}, nil
}

func kindOf(name string) int {
	for kind, n := range kindNames {
		if n == name {
			return kind
		}
	}
	return 0
}

// packList answers the packs, scanning the directory on first use.
func (db *odb) packList(force bool) []*pack {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.scanned && !force {
		return db.packs
	}
	dir := filepath.Join(db.dir, "pack")
	db.stamp = stampOf(dir)
	known := map[string]*pack{}
	for _, p := range db.packs {
		known[p.path] = p
	}
	var out []*pack
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		base, ok := strings.CutSuffix(e.Name(), ".idx")
		if !ok || e.IsDir() {
			continue
		}
		path := filepath.Join(dir, base)
		if _, err := os.Stat(path + ".pack"); err != nil {
			continue
		}
		if p, ok := known[path]; ok {
			out = append(out, p)
			continue
		}
		out = append(out, &pack{path: path})
	}
	db.packs, db.scanned = out, true
	return out
}

// packsChanged rescans the pack directory when its stamp moved.
func (db *odb) packsChanged() bool {
	db.mu.Lock()
	moved := db.stamp != stampOf(filepath.Join(db.dir, "pack"))
	db.mu.Unlock()
	if moved {
		db.packList(true)
	}
	return moved
}

// alternates reads objects/info/alternates once.
func (db *odb) alternates() []*odb {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.altsRead {
		return db.alts
	}
	db.altsRead = true
	file, err := os.Open(filepath.Join(db.dir, "info", "alternates"))
	if err != nil {
		return nil
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !filepath.IsAbs(line) {
			line = filepath.Join(db.dir, line)
		}
		db.alts = append(db.alts, sharedODB(filepath.Clean(line)))
	}
	return db.alts
}

// odbs shares one object directory between every repository that reads it.
var odbs sync.Map

func sharedODB(dir string) *odb {
	if v, ok := odbs.Load(dir); ok {
		return v.(*odb)
	}
	v, _ := odbs.LoadOrStore(dir, newODB(dir))
	return v.(*odb)
}

// fileStamp tells a rewritten file from the read before.
type fileStamp struct {
	size  int64
	mtime int64
	ok    bool
}

func stampOf(path string) fileStamp {
	info, err := os.Lstat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{size: info.Size(), mtime: info.ModTime().UnixNano(), ok: true}
}
