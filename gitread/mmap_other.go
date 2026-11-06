//go:build !unix

// mapFile opens a file for reads at an offset, where no mmap is portable.

package gitread

import "os"

// mapFile opens a file for reads at an offset, where no mmap is portable.
func mapFile(path string) (fileData, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	return &fileReader{file: file, size: info.Size()}, nil
}
