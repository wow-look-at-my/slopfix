// Object kinds, as a pack numbers them.

package gitread

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
)

// Object kinds, as a pack numbers them.
const (
	kindCommit   = 1
	kindTree     = 2
	kindBlob     = 3
	kindTag      = 4
	kindOfsDelta = 6
	kindRefDelta = 7
)

// fanoutSize is the byte length of the fanout table of a pack index.
const fanoutSize = 256 * 4

// pack is one packfile and its index, both read in place.
type pack struct {
	path string

	once sync.Once
	err  error
	idx  fileData
	data fileData
	// count is the number of objects the index names.
	count int
	// fanout holds, for each first byte, how many names sort at or below it.
	fanout []byte
}

// open maps the index and the pack on first use.
func (p *pack) open() error {
	p.once.Do(func() {
		idx, err := mapFile(p.path + ".idx")
		if err != nil {
			p.err = err
			return
		}
		head, err := idx.Bytes(0, 8+fanoutSize)
		if err != nil || !bytes.Equal(head[:4], []byte{0xff, 't', 'O', 'c'}) || binary.BigEndian.Uint32(head[4:8]) != 2 {
			p.err = fmt.Errorf("gitread: %s.idx is not a version 2 pack index", p.path)
			return
		}
		p.fanout = head[8:]
		p.count = int(binary.BigEndian.Uint32(p.fanout[fanoutSize-4:]))
		if idx.Len() < int64(8+fanoutSize+p.count*(20+4+4)) {
			p.err = fmt.Errorf("gitread: %s.idx is truncated", p.path)
			return
		}
		data, err := mapFile(p.path + ".pack")
		if err != nil {
			p.err = err
			return
		}
		p.idx, p.data = idx, data
	})
	return p.err
}

func (p *pack) fan(i int) int {
	if i < 0 {
		return 0
	}
	return int(binary.BigEndian.Uint32(p.fanout[i*4:]))
}

// name answers the object name at position i of the index.
func (p *pack) name(i int) []byte {
	b, _ := p.idx.Bytes(int64(8+fanoutSize+i*20), 20)
	return b
}

// find answers the pack offset of an object, or false.
func (p *pack) find(oid OID) (int64, bool) {
	if p.open() != nil {
		return 0, false
	}
	lo, hi := p.fan(int(oid[0])-1), p.fan(int(oid[0]))
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		switch bytes.Compare(p.name(mid), oid[:]) {
		case 0:
			return p.offset(mid)
		case -1:
			lo = mid + 1
		default:
			hi = mid
		}
	}
	return 0, false
}

// offset reads the pack offset at position i of the index.
func (p *pack) offset(i int) (int64, bool) {
	base := int64(8 + fanoutSize + p.count*24)
	raw, err := p.idx.Bytes(base+int64(i)*4, 4)
	if err != nil {
		return 0, false
	}
	v := binary.BigEndian.Uint32(raw)
	if v&0x80000000 == 0 {
		return int64(v), true
	}
	large, err := p.idx.Bytes(base+int64(p.count)*4+int64(v&0x7fffffff)*8, 8)
	if err != nil {
		return 0, false
	}
	return int64(binary.BigEndian.Uint64(large)), true
}

// prefix answers each object name in the index that starts with the hex digits.
func (p *pack) prefix(digits []byte, odd bool, visit func(OID)) {
	if p.open() != nil || len(digits) == 0 {
		return
	}
	lo, hi := p.fan(int(digits[0])-1), p.fan(int(digits[0]))
	for i := lo; i < hi; i++ {
		name := p.name(i)
		if hasPrefix(name, digits, odd) {
			visit(oidOf(name))
		}
	}
}

// hasPrefix compares a name with packed hex digits. With odd, the last digit
// fills only the high half of its byte.
func hasPrefix(name, digits []byte, odd bool) bool {
	full := len(digits)
	if odd {
		full--
	}
	if !bytes.Equal(name[:full], digits[:full]) {
		return false
	}
	return !odd || name[full]&0xf0 == digits[full]&0xf0
}

// header reads a pack object header: kind and inflated size.
func header(r byteReader) (kind int, size uint64, err error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	kind = int(b>>4) & 7
	size = uint64(b & 0x0f)
	shift := uint(4)
	for b&0x80 != 0 {
		if b, err = r.ReadByte(); err != nil {
			return 0, 0, err
		}
		size |= uint64(b&0x7f) << shift
		shift += 7
	}
	return kind, size, nil
}

// ofsDistance reads how far back an offset delta's base starts.
func ofsDistance(r byteReader) (int64, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	value := int64(b & 0x7f)
	for b&0x80 != 0 {
		if b, err = r.ReadByte(); err != nil {
			return 0, err
		}
		value = (value+1)<<7 | int64(b&0x7f)
	}
	return value, nil
}

// deltaLink is one step of a delta chain: where it sits and what it inflates to.
type deltaLink struct {
	off   int64
	delta []byte
}

// baseKey names a resolved object inside one pack.
type baseKey struct {
	p   *pack
	off int64
}

// read resolves the object at off. It walks the delta chain down to a base it
// already holds, then applies the deltas upward and keeps each result.
func (p *pack) read(db *odb, off int64) (object, error) {
	var chain []deltaLink
	cur := off
	var base object
	for {
		if obj, ok := db.bases.get(baseKey{p, cur}); ok {
			base = obj
			break
		}
		r := p.data.Reader(cur)
		kind, size, err := header(r)
		if err != nil {
			return object{}, fmt.Errorf("gitread: %s: object header at %d: %w", p.path, cur, err)
		}
		switch kind {
		case kindCommit, kindTree, kindBlob, kindTag:
			data, err := inflate(r, size)
			if err != nil {
				return object{}, err
			}
			base = object{kind: kind, data: data}
		case kindOfsDelta:
			dist, err := ofsDistance(r)
			if err != nil {
				return object{}, err
			}
			delta, err := inflate(r, size)
			if err != nil {
				return object{}, err
			}
			chain = append(chain, deltaLink{cur, delta})
			cur -= dist
			continue
		case kindRefDelta:
			var name OID
			for i := range name {
				if name[i], err = r.ReadByte(); err != nil {
					return object{}, err
				}
			}
			delta, err := inflate(r, size)
			if err != nil {
				return object{}, err
			}
			chain = append(chain, deltaLink{cur, delta})
			if at, ok := p.find(name); ok {
				cur = at
				continue
			}
			if base, err = db.object(name); err != nil {
				return object{}, err
			}
		default:
			return object{}, fmt.Errorf("gitread: %s: object kind %d at %d is unknown", p.path, kind, cur)
		}
		break
	}
	if len(chain) > 0 {
		db.bases.put(baseKey{p, cur}, base, len(base.data))
	}
	for i := len(chain) - 1; i >= 0; i-- {
		data, err := applyDelta(base.data, chain[i].delta)
		if err != nil {
			return object{}, fmt.Errorf("gitread: %s at %d: %w", p.path, chain[i].off, err)
		}
		base = object{kind: base.kind, data: data}
		if i > 0 {
			db.bases.put(baseKey{p, chain[i].off}, base, len(base.data))
		}
	}
	return base, nil
}

// applyDelta rebuilds a target from its base and a delta.
func applyDelta(base, delta []byte) ([]byte, error) {
	pos := 0
	size := func() (uint64, error) {
		var v uint64
		var shift uint
		for {
			if pos >= len(delta) {
				return 0, fmt.Errorf("the delta ends inside a size")
			}
			b := delta[pos]
			pos++
			v |= uint64(b&0x7f) << shift
			shift += 7
			if b&0x80 == 0 {
				return v, nil
			}
		}
	}
	baseSize, err := size()
	if err != nil {
		return nil, err
	}
	if baseSize != uint64(len(base)) {
		return nil, fmt.Errorf("the delta names a base of %d bytes, but the base has %d", baseSize, len(base))
	}
	targetSize, err := size()
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, targetSize)
	for pos < len(delta) {
		op := delta[pos]
		pos++
		if op&0x80 == 0 {
			if op == 0 || pos+int(op) > len(delta) {
				return nil, fmt.Errorf("the delta holds a bad insert")
			}
			out = append(out, delta[pos:pos+int(op)]...)
			pos += int(op)
			continue
		}
		var off, n uint64
		for i := uint(0); i < 7; i++ {
			if op&(1<<i) == 0 {
				continue
			}
			if pos >= len(delta) {
				return nil, fmt.Errorf("the delta ends inside a copy")
			}
			if i < 4 {
				off |= uint64(delta[pos]) << (8 * i)
			} else {
				n |= uint64(delta[pos]) << (8 * (i - 4))
			}
			pos++
		}
		if n == 0 {
			n = 0x10000
		}
		if off+n > uint64(len(base)) {
			return nil, fmt.Errorf("the delta copies past the base")
		}
		out = append(out, base[off:off+n]...)
	}
	if uint64(len(out)) != targetSize {
		return nil, fmt.Errorf("the delta built %d bytes, not %d", len(out), targetSize)
	}
	return out, nil
}
