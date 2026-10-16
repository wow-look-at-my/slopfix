// Package gitread reads a git repository in-process.
//
// The rules ask many small questions of one repository: is this path ignored,
// what attributes does it carry, what did this blob hold at the base. Each
// question as a git subprocess costs a fork and an exec. This package answers
// them from the files git writes, and keeps what it reads for the next caller.
package gitread

import (
	"crypto/sha1"
	"fmt"
	"strconv"
)

// OID is a SHA-1 object name.
type OID [20]byte

// String answers the hexadecimal form.
func (o OID) String() string {
	const hex = "0123456789abcdef"
	var out [40]byte
	for i, b := range o {
		out[i*2] = hex[b>>4]
		out[i*2+1] = hex[b&0x0f]
	}
	return string(out[:])
}

// IsZero reports whether the object name is the zero value.
func (o OID) IsZero() bool { return o == OID{} }

// ParseOID reads a full hexadecimal object name.
func ParseOID(s string) (OID, error) {
	var out OID
	if len(s) != len(out)*2 {
		return out, fmt.Errorf("gitread: %q is not a full object name", s)
	}
	for i := range out {
		hi, ok1 := hexVal(s[i*2])
		lo, ok2 := hexVal(s[i*2+1])
		if !ok1 || !ok2 {
			return out, fmt.Errorf("gitread: %q is not a full object name", s)
		}
		out[i] = hi<<4 | lo
	}
	return out, nil
}

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func oidOf(b []byte) OID {
	var out OID
	copy(out[:], b)
	return out
}

// HashBlob answers the name git gives a blob with this content.
func HashBlob(data []byte) OID {
	h := sha1.New()
	h.Write([]byte("blob "))
	h.Write([]byte(strconv.Itoa(len(data))))
	h.Write([]byte{0})
	h.Write(data)
	return oidOf(h.Sum(nil))
}
