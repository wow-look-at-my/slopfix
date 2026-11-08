// message.go holds the message code.

package twelvehour

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Message rewrites one flush of a streamed message. A marker file named for
// the message under dir records that the last flush ended inside a fence. The
// final flush removes it.
func Message(dir, messageID, text string, final bool, now time.Time) (string, error) {
	marker := filepath.Join(dir, filepath.Base(messageID))
	_, err := os.Stat(marker)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Rewrite(text, now), err
	}
	out, inFence := rewriteFrom(text, now, err == nil)
	if inFence && !final {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return out, err
		}
		return out, os.WriteFile(marker, nil, 0o600)
	}
	if err := os.Remove(marker); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return out, err
	}
	return out, nil
}
