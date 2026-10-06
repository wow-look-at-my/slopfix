package gitread

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// IndexEntry is one entry of the git index.
type IndexEntry struct {
	Path  string
	OID   OID
	Mode  string
	Stage int
}

// Index is the parsed git index.
type Index struct {
	entries []IndexEntry
	byPath  map[string]IndexEntry
}

// Entries answers every index entry.
func (i *Index) Entries() []IndexEntry { return i.entries }

// Tracked answers the stage-zero entry of each path.
func (i *Index) Tracked() map[string]IndexEntry { return i.byPath }

// Index reads the work tree's index, cached.
func (r *Repo) Index() (*Index, error) {
	r.mu.Lock()
	if r.idx != nil {
		r.mu.Unlock()
		return r.idx, nil
	}
	r.mu.Unlock()
	idx, err := readIndexFile(filepath.Join(r.gitDir, "index"))
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.idx = idx
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
		return nil, fmt.Errorf("gitread: index version %d is not supported", version)
	}
	count := int(binary.BigEndian.Uint32(data[8:12]))
	pos := 12
	out := &Index{byPath: map[string]IndexEntry{}}
	prev := ""
	for n := 0; n < count; n++ {
		start := pos
		if pos+62 > len(data) {
			return nil, fmt.Errorf("gitread: the index ends inside an entry")
		}
		pos += 40
		entry := IndexEntry{OID: oidOf(data[pos : pos+20])}
		pos += 20
		flags := binary.BigEndian.Uint16(data[pos : pos+2])
		pos += 2
		entry.Stage = int(flags>>12) & 3
		if flags&0x4000 != 0 {
			if pos+2 > len(data) {
				return nil, fmt.Errorf("gitread: the index ends inside extended flags")
			}
			pos += 2
		}
		if version == 4 {
			strip, n, err := decodeVarint(data[pos:])
			if err != nil {
				return nil, err
			}
			pos += n
			keep := len(prev) - int(strip)
			if keep < 0 {
				return nil, fmt.Errorf("gitread: index entry strips past the previous path")
			}
			end := bytes.IndexByte(data[pos:], 0)
			if end < 0 {
				return nil, fmt.Errorf("gitread: index entry name is unterminated")
			}
			entry.Path = prev[:keep] + string(data[pos:pos+end])
			pos += end + 1
		} else {
			end := bytes.IndexByte(data[pos:], 0)
			if end < 0 {
				return nil, fmt.Errorf("gitread: index entry name is unterminated")
			}
			entry.Path = string(data[pos : pos+end])
			pos += end + 1
			// Entries are padded to a multiple of several bytes.
			for (pos-start)%8 != 0 {
				pos++
			}
		}
		entry.Mode = strconv.FormatUint(uint64(binary.BigEndian.Uint32(data[start+24:start+28])), 8)
		prev = entry.Path
		out.entries = append(out.entries, entry)
		if entry.Stage == 0 {
			out.byPath[entry.Path] = entry
		}
	}
	return out, nil
}

// decodeVarint reads git's pack-style varint, answering the value and its length.
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
		value = (value << 7) | uint64(c&0x7f)
	}
	return value, i, nil
}
