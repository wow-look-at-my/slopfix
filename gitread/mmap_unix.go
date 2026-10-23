//go:build unix

// mapFile maps a whole file read-only.

package gitread

import (
	"os"
	"syscall"
)

// mapFile maps a whole file read-only. The mapping outlives a later repack,
// because an unlinked file stays mapped.
func mapFile(path string) (fileData, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()
	if size == 0 {
		return bytesData(nil), nil
	}
	data, err := syscall.Mmap(int(file.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return readWhole(path)
	}
	return bytesData(data), nil
}
