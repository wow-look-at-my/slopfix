// byteReader is what the pack decoder and the inflater read from.

package gitread

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/klauspost/compress/zlib"
)

// byteReader is what the pack decoder and the inflater read from.
type byteReader interface {
	io.Reader
	io.ByteReader
}

// fileData is a read-only file: a mapping, or an open file read at offsets.
type fileData interface {
	Len() int64
	// Bytes answers n bytes at off. A mapping answers a view, which callers never write.
	Bytes(off, n int64) ([]byte, error)
	Reader(off int64) byteReader
}

type bytesData []byte

func (d bytesData) Len() int64 { return int64(len(d)) }

func (d bytesData) Bytes(off, n int64) ([]byte, error) {
	if off < 0 || n < 0 || off+n > int64(len(d)) {
		return nil, fmt.Errorf("gitread: a read runs past the end of a file")
	}
	return d[off : off+n], nil
}

func (d bytesData) Reader(off int64) byteReader {
	if off < 0 || off > int64(len(d)) {
		off = int64(len(d))
	}
	return bytes.NewReader(d[off:])
}

// fileReader reads an open file at offsets. Each read is one pread.
type fileReader struct {
	file *os.File
	size int64
}

func (f *fileReader) Len() int64 { return f.size }

func (f *fileReader) Bytes(off, n int64) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := f.file.ReadAt(buf, off); err != nil {
		return nil, err
	}
	return buf, nil
}

func (f *fileReader) Reader(off int64) byteReader {
	return bufio.NewReader(io.NewSectionReader(f.file, off, f.size-off))
}

// readWhole reads a file into memory where a mapping fails.
func readWhole(path string) (fileData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytesData(data), nil
}

// inflaters keeps zlib readers for reuse. A new reader allocates its window.
var inflaters sync.Pool

// inflater answers a pooled zlib reader set to read r.
func inflater(r byteReader) (io.ReadCloser, error) {
	if pooled, ok := inflaters.Get().(io.ReadCloser); ok {
		if err := pooled.(zlib.Resetter).Reset(r, nil); err != nil {
			return nil, fmt.Errorf("gitread: inflate: %w", err)
		}
		return pooled, nil
	}
	fresh, err := zlib.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gitread: inflate: %w", err)
	}
	return fresh, nil
}

// inflate reads a zlib stream that holds exactly size bytes.
func inflate(r byteReader, size uint64) ([]byte, error) {
	zr, err := inflater(r)
	if err != nil {
		return nil, err
	}
	data := make([]byte, size)
	_, err = io.ReadFull(zr, data)
	inflaters.Put(zr)
	if err != nil {
		return nil, fmt.Errorf("gitread: inflate: %w", err)
	}
	return data, nil
}

// inflateAll reads a whole zlib stream.
func inflateAll(r byteReader) ([]byte, error) {
	zr, err := inflater(r)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(zr)
	inflaters.Put(zr)
	if err != nil {
		return nil, fmt.Errorf("gitread: inflate: %w", err)
	}
	return data, nil
}
