// Package gitread reads a git repository in-process.
//
// A fork check asks many small questions of one repository: does this path
// exist at the base commit, what did that blob hold. Which lines differ. Each
// question as a git subprocess is a fork and an exec, thousands of times over.
// This package answers them from the object store itself, so no git process is
// started.
//
// It reads loose objects and packfiles with their index, resolves delta
// chains, walks commits, flattens trees, reads the index, and applies
// gitignore and gitattributes rules.
package gitread

import (
	"fmt"
)

// OID is a SHA-1 object name.
type OID [20]byte

// String answers the 40-character hexadecimal form.
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

// ParseOID reads a 40-character hexadecimal object name.
func ParseOID(s string) (OID, error) {
	var out OID
	if len(s) != 40 {
		return out, fmt.Errorf("gitread: %q is not a 40-character object name", s)
	}
	for i := 0; i < 20; i++ {
		hi, ok1 := hexVal(s[i*2])
		lo, ok2 := hexVal(s[i*2+1])
		if !ok1 || !ok2 {
			return out, fmt.Errorf("gitread: %q is not a 40-character object name", s)
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

func looksLikeOID(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < 40; i++ {
		if _, ok := hexVal(s[i]); !ok {
			return false
		}
	}
	return true
}
