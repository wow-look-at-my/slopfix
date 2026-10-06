package gitread

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Type names, as git spells them.
const (
	TypeCommit = "commit"
	TypeTree   = "tree"
	TypeBlob   = "blob"
	TypeTag    = "tag"
)

// Object answers the type and content of an object.
func (r *Repo) Object(oid OID) (string, []byte, error) {
	obj, err := r.object(oid)
	if err != nil {
		return "", nil, err
	}
	return obj.typ, obj.data, nil
}

// Has reports whether the object is in this repository.
func (r *Repo) Has(oid OID) bool {
	_, err := r.object(oid)
	return err == nil
}

// object answers an object from the cache, a loose file or a packfile.
func (r *Repo) object(oid OID) (object, error) {
	r.mu.Lock()
	if obj, ok := r.objs[oid]; ok {
		r.mu.Unlock()
		return obj, nil
	}
	r.mu.Unlock()

	obj, err := r.readLoose(oid)
	if err != nil {
		obj, err = r.readPacked(oid)
	}
	if err != nil {
		r.mu.Lock()
		extra := r.extra
		r.mu.Unlock()
		for _, other := range extra {
			if found, err := other.object(oid); err == nil {
				obj, err = found, nil
				break
			}
		}
	}
	if err != nil {
		return object{}, err
	}
	r.mu.Lock()
	r.objs[oid] = obj
	r.mu.Unlock()
	return obj, nil
}

// readLoose inflates the object from objects/xx/yyyy...
func (r *Repo) readLoose(oid OID) (object, error) {
	hex := oid.String()
	path := filepath.Join(r.commonDir, "objects", hex[:2], hex[2:])
	file, err := os.Open(path)
	if err != nil {
		return object{}, errObjectMissing
	}
	defer file.Close()
	zr, err := zlib.NewReader(file)
	if err != nil {
		return object{}, fmt.Errorf("gitread: %s is not a zlib stream: %w", hex, err)
	}
	defer zr.Close()
	raw, err := io.ReadAll(zr)
	if err != nil {
		return object{}, fmt.Errorf("gitread: read the loose object %s: %w", hex, err)
	}
	typ, data, err := splitHeader(raw)
	if err != nil {
		return object{}, err
	}
	return object{typ: typ, data: data}, nil
}

// splitHeader cuts the "type size\0" header off an object's content.
func splitHeader(raw []byte) (string, []byte, error) {
	nul := bytes.IndexByte(raw, 0)
	if nul < 0 {
		return "", nil, fmt.Errorf("gitread: an object has no header")
	}
	header := string(raw[:nul])
	space := strings.IndexByte(header, ' ')
	if space < 0 {
		return "", nil, fmt.Errorf("gitread: %q is not an object header", header)
	}
	typ := header[:space]
	if typ != TypeCommit && typ != TypeTree && typ != TypeBlob && typ != TypeTag {
		return "", nil, fmt.Errorf("gitread: %q is not an object type", typ)
	}
	return typ, raw[nul+1:], nil
}

// readPacked answers an object from the packfiles, loading their indexes once.
func (r *Repo) readPacked(oid OID) (object, error) {
	packs, err := r.packFiles()
	if err != nil {
		return object{}, err
	}
	for _, p := range packs {
		off, ok := p.idx.find(oid)
		if !ok {
			continue
		}
		typ, data, err := p.at(off)
		if err != nil {
			return object{}, err
		}
		return object{typ: typ, data: data}, nil
	}
	return object{}, errObjectMissing
}

// packFiles answers the packfiles, opening each index once.
func (r *Repo) packFiles() ([]*pack, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.packs != nil {
		return r.packs, nil
	}
	dir := filepath.Join(r.commonDir, "objects", "pack")
	entries, err := os.ReadDir(dir)
	if err != nil {
		r.packs = []*pack{}
		return r.packs, nil
	}
	var out []*pack
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".idx") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".idx")
		idx, err := readIndex(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, &pack{repo: r, path: filepath.Join(dir, base+".pack"), idx: idx, cache: map[uint64]object{}})
	}
	r.packs = out
	return out, nil
}

// pack is one packfile with its index.
type pack struct {
	repo  *Repo
	path  string
	idx   *packIndex
	cache map[uint64]object
}

// at answers the object at an offset, resolving its delta chain.
func (p *pack) at(off uint64) (string, []byte, error) {
	if obj, ok := p.cache[off]; ok {
		return obj.typ, obj.data, nil
	}
	file, err := os.Open(p.path)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()
	if _, err := file.Seek(int64(off), io.SeekStart); err != nil {
		return "", nil, err
	}
	br := bufio.NewReader(file)
	typ, size, err := readObjHeader(br)
	if err != nil {
		return "", nil, err
	}
	switch typ {
	case 1, 2, 3, 4:
		data, err := inflate(br, size)
		if err != nil {
			return "", nil, err
		}
		name := map[int]string{1: TypeCommit, 2: TypeTree, 3: TypeBlob, 4: TypeTag}[typ]
		p.cache[off] = object{typ: name, data: data}
		return name, data, nil
	case 6:
		neg, err := readOfsDelta(br)
		if err != nil {
			return "", nil, err
		}
		baseTyp, baseData, err := p.at(off - neg)
		if err != nil {
			return "", nil, err
		}
		delta, err := inflate(br, size)
		if err != nil {
			return "", nil, err
		}
		data, err := applyDelta(baseData, delta)
		if err != nil {
			return "", nil, err
		}
		p.cache[off] = object{typ: baseTyp, data: data}
		return baseTyp, data, nil
	case 7:
		var base OID
		if _, err := io.ReadFull(br, base[:]); err != nil {
			return "", nil, err
		}
		delta, err := inflate(br, size)
		if err != nil {
			return "", nil, err
		}
		baseObj, err := p.repoObject(base)
		if err != nil {
			return "", nil, err
		}
		data, err := applyDelta(baseObj.data, delta)
		if err != nil {
			return "", nil, err
		}
		p.cache[off] = object{typ: baseObj.typ, data: data}
		return baseObj.typ, data, nil
	}
	return "", nil, fmt.Errorf("gitread: pack object type %d is unknown", typ)
}

// repoObject answers a ref-delta's base from the repository.
func (p *pack) repoObject(oid OID) (object, error) {
	return p.repo.object(oid)
}

// readObjHeader reads a pack object header: type and inflated size.
func readObjHeader(br *bufio.Reader) (typ int, size uint64, err error) {
	b, err := br.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	typ = int(b>>4) & 7
	size = uint64(b & 0x0f)
	shift := uint(4)
	for b&0x80 != 0 {
		b, err = br.ReadByte()
		if err != nil {
			return 0, 0, err
		}
		size |= uint64(b&0x7f) << shift
		shift += 7
	}
	return typ, size, nil
}

// readOfsDelta reads the negative offset of an ofs-delta.
func readOfsDelta(br *bufio.Reader) (uint64, error) {
	b, err := br.ReadByte()
	if err != nil {
		return 0, err
	}
	value := uint64(b & 0x7f)
	for b&0x80 != 0 {
		b, err = br.ReadByte()
		if err != nil {
			return 0, err
		}
		value = (value + 1)<<7 | uint64(b&0x7f)
	}
	return value, nil
}

// inflate reads a zlib stream of the given inflated size.
func inflate(br *bufio.Reader, size uint64) ([]byte, error) {
	zr, err := zlib.NewReader(br)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	data := make([]byte, size)
	if _, err := io.ReadFull(zr, data); err != nil {
		return nil, fmt.Errorf("gitread: inflate: %w", err)
	}
	return data, nil
}

// applyDelta rebuilds a target from its base and a delta.
func applyDelta(base, delta []byte) ([]byte, error) {
	pos := 0
	readSize := func() (uint64, error) {
		var size uint64
		var shift uint
		for {
			if pos >= len(delta) {
				return 0, fmt.Errorf("gitread: the delta ends inside a size")
			}
			b := delta[pos]
			pos++
			size |= uint64(b&0x7f) << shift
			shift += 7
			if b&0x80 == 0 {
				return size, nil
			}
		}
	}
	baseSize, err := readSize()
	if err != nil {
		return nil, err
	}
	if baseSize != uint64(len(base)) {
		return nil, fmt.Errorf("gitread: the delta names a base of %d bytes, but the base has %d", baseSize, len(base))
	}
	targetSize, err := readSize()
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, targetSize)
	for pos < len(delta) {
		op := delta[pos]
		pos++
		if op&0x80 != 0 {
			var off, size uint64
			for i := 0; i < 4; i++ {
				if op&(1<<uint(i)) != 0 {
					if pos >= len(delta) {
						return nil, fmt.Errorf("gitread: the delta ends inside a copy offset")
					}
					off |= uint64(delta[pos]) << (8 * uint(i))
					pos++
				}
			}
			for i := 0; i < 3; i++ {
				if op&(1<<uint(4+i)) != 0 {
					if pos >= len(delta) {
						return nil, fmt.Errorf("gitread: the delta ends inside a copy size")
					}
					size |= uint64(delta[pos]) << (8 * uint(i))
					pos++
				}
			}
			if size == 0 {
				size = 0x10000
			}
			if off+size > uint64(len(base)) {
				return nil, fmt.Errorf("gitread: the delta copies past the base")
			}
			out = append(out, base[off:off+size]...)
			continue
		}
		if op == 0 {
			return nil, fmt.Errorf("gitread: the delta holds a zero opcode")
		}
		if pos+int(op) > len(delta) {
			return nil, fmt.Errorf("gitread: the delta ends inside an insert")
		}
		out = append(out, delta[pos:pos+int(op)]...)
		pos += int(op)
	}
	if uint64(len(out)) != targetSize {
		return nil, fmt.Errorf("gitread: the delta built %d bytes, not %d", len(out), targetSize)
	}
	return out, nil
}

// packIndex maps object names to pack offsets.
type packIndex struct {
	oids    []OID
	offsets []uint64
}

// find answers the offset of an object name.
func (p *packIndex) find(oid OID) (uint64, bool) {
	lo, hi := 0, len(p.oids)
	for lo < hi {
		mid := (lo + hi) / 2
		switch bytes.Compare(p.oids[mid][:], oid[:]) {
		case 0:
			return p.offsets[mid], true
		case -1:
			lo = mid + 1
		default:
			hi = mid
		}
	}
	return 0, false
}

// readIndex reads a version 2 pack index.
func readIndex(path string) (*packIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 8+256*4 {
		return nil, fmt.Errorf("gitread: %s is too short for a pack index", path)
	}
	if !bytes.Equal(data[:4], []byte{0xff, 't', 'O', 'c'}) || binary.BigEndian.Uint32(data[4:8]) != 2 {
		return nil, fmt.Errorf("gitread: %s is not a version 2 pack index", path)
	}
	fanout := data[8 : 8+256*4]
	count := int(binary.BigEndian.Uint32(fanout[255*4:]))
	base := 8 + 256*4
	need := base + count*(20+4+4)
	if len(data) < need {
		return nil, fmt.Errorf("gitread: %s is truncated", path)
	}
	idx := &packIndex{oids: make([]OID, count), offsets: make([]uint64, count)}
	sha := data[base : base+count*20]
	for i := 0; i < count; i++ {
		idx.oids[i] = oidOf(sha[i*20 : i*20+20])
	}
	offBase := base + count*20 + count*4
	large := offBase + count*4
	for i := 0; i < count; i++ {
		v := binary.BigEndian.Uint32(data[offBase+i*4:])
		if v&0x80000000 != 0 {
			li := int(v & 0x7fffffff)
			start := large + li*8
			if start+8 > len(data) {
				return nil, fmt.Errorf("gitread: %s has a large offset past its end", path)
			}
			idx.offsets[i] = binary.BigEndian.Uint64(data[start:])
			continue
		}
		idx.offsets[i] = uint64(v)
	}
	return idx, nil
}
