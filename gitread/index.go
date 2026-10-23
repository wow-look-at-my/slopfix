// Index entry flags.

package gitread

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
)

// Index entry flags.
const (
	flagAssumeValid  = 0x8000
	flagExtended     = 0x4000
	flagSkipWorktree = 0x4000
)

// Mode bits an index entry carries.
const (
	modeGitlink = 0o160000
	modeSymlink = 0o120000
	modeExec    = 0o100755
	modeFile    = 0o100644
)

// IndexEntry is one entry of the git index.
type IndexEntry struct {
	Path  string
	OID   OID
	Mode  string
	Stage int

	mode     uint32
	mtimeNS  int64
	size     uint32
	unstaged bool
}

// IsGitlink reports a submodule entry.
func (e IndexEntry) IsGitlink() bool { return e.mode == modeGitlink }

// Index is the parsed git index.
type Index struct {
	entries []IndexEntry
	byPath  map[string]IndexEntry
	// mtimeNS is when the index was written. An entry no older than it may hide an edit.
	mtimeNS int64
}

// Entries answers every index entry, in index order.
func (i *Index) Entries() []IndexEntry { return i.entries }

// Tracked answers the stage-zero entry of each path.
func (i *Index) Tracked() map[string]IndexEntry { return i.byPath }

// Index reads the index, again only when its file changed.
func (r *Repo) Index() (*Index, error) {
	stamp := stampOf(r.indexFile)
	r.mu.Lock()
	if r.idx != nil && stamp == r.idxStamp {
		idx := r.idx
		r.mu.Unlock()
		return idx, nil
	}
	r.mu.Unlock()
	idx, err := readIndexFile(r.indexFile)
	if os.IsNotExist(err) {
		// A repository with nothing staged yet holds no index.
		idx, err = &Index{byPath: map[string]IndexEntry{}}, nil
	}
	if err != nil {
		return nil, err
	}
	idx.mtimeNS = stamp.mtime
	r.mu.Lock()
	r.idx, r.idxStamp = idx, stamp
	r.mu.Unlock()
	return idx, nil
}

func readIndexFile(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 12 || !bytes.Equal(data[:4], []byte("DIRC")) {
		return nil, fmt.Errorf("gitread: %s is not a git index", path)
	}
	version := binary.BigEndian.Uint32(data[4:8])
	if version < 2 || version > 4 {
		return nil, fmt.Errorf("%w: index version %d", ErrUnsupported, version)
	}
	count := int(binary.BigEndian.Uint32(data[8:12]))
	pos := 12
	out := &Index{entries: make([]IndexEntry, 0, count), byPath: make(map[string]IndexEntry, count)}
	prev := ""
	for n := 0; n < count; n++ {
		start := pos
		if pos+62 > len(data) {
			return nil, fmt.Errorf("gitread: %s ends inside an entry", path)
		}
		be := binary.BigEndian
		entry := IndexEntry{
			mtimeNS: int64(be.Uint32(data[pos+8:]))*1e9 + int64(be.Uint32(data[pos+12:])),
			mode:    be.Uint32(data[pos+24:]),
			size:    be.Uint32(data[pos+36:]),
			OID:     oidOf(data[pos+40 : pos+60]),
		}
		flags := be.Uint16(data[pos+60:])
		pos += 62
		entry.Stage = int(flags>>12) & 3
		entry.unstaged = flags&flagAssumeValid != 0
		if flags&flagExtended != 0 {
			if pos+2 > len(data) {
				return nil, fmt.Errorf("gitread: %s ends inside extended flags", path)
			}
			ext := be.Uint16(data[pos:])
			entry.unstaged = entry.unstaged || ext&flagSkipWorktree != 0
			pos += 2
		}
		end := bytes.IndexByte(data[pos:], 0)
		if end < 0 {
			return nil, fmt.Errorf("gitread: %s holds an unterminated path", path)
		}
		if version == 4 {
			strip, used, err := decodeVarint(data[pos:])
			if err != nil {
				return nil, err
			}
			pos += used
			keep := len(prev) - int(strip)
			if keep < 0 {
				return nil, fmt.Errorf("gitread: %s strips past the previous path", path)
			}
			end = bytes.IndexByte(data[pos:], 0)
			if end < 0 {
				return nil, fmt.Errorf("gitread: %s holds an unterminated path", path)
			}
			entry.Path = prev[:keep] + string(data[pos:pos+end])
			pos += end + 1
		} else {
			entry.Path = string(data[pos : pos+end])
			pos += end + 1
			// An entry pads to a multiple of several bytes.
			pos = start + (pos-start+7)&^7
		}
		if entry.mode&0o170000 == 0o040000 {
			return nil, fmt.Errorf("%w: %s is a sparse index", ErrUnsupported, path)
		}
		entry.Mode = strconv.FormatUint(uint64(entry.mode), 8)
		prev = entry.Path
		out.entries = append(out.entries, entry)
		if entry.Stage == 0 {
			out.byPath[entry.Path] = entry
		}
	}
	if err := checkExtensions(data[pos:], path); err != nil {
		return nil, err
	}
	return out, nil
}

// checkExtensions refuses a split index, whose entries live in another file.
func checkExtensions(data []byte, path string) error {
	for len(data) >= 8+20 {
		sig := string(data[:4])
		size := int(binary.BigEndian.Uint32(data[4:8]))
		if sig == "link" {
			return fmt.Errorf("%w: %s is a split index", ErrUnsupported, path)
		}
		if 8+size > len(data) {
			return nil
		}
		data = data[8+size:]
	}
	return nil
}

// decodeVarint reads git's offset varint, answering the value and its length.
func decodeVarint(data []byte) (uint64, int, error) {
	if len(data) == 0 {
		return 0, 0, fmt.Errorf("gitread: a varint is empty")
	}
	i := 0
	c := data[i]
	i++
	value := uint64(c & 0x7f)
	for c&0x80 != 0 {
		value++
		if i >= len(data) {
			return 0, 0, fmt.Errorf("gitread: a varint is truncated")
		}
		c = data[i]
		i++
		value = value<<7 | uint64(c&0x7f)
	}
	return value, i, nil
}
