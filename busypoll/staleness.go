// staleness.go bounds how long a single observation keeps a subject shut. A
// verdict can be partial -- checks reported, a single still running -- and
// even a whole a single stops being the answer a single time the model waits
// on something that read could not see. So evidence ages out, and the
// subject re-opens by itself after the same window that separates a busy-poll from a real wait.
package busypoll

import "time"

// recentRecords returns the records written inside the window, dropping the
// older ones. It cuts a prefix, because records arrive in order, and a record
// with no timestamp keeps the position its neighbours give it. A transcript
// that carries no usable timestamp at all is returned whole: an unreadable
// clock must not turn this rule off, and it must not turn it into a block.
func recentRecords(recs []record) []record {
	var newest time.Time
	for _, r := range recs {
		if r.at.After(newest) {
			newest = r.at
		}
	}
	if newest.IsZero() {
		return recs
	}
	cutoff := time.Now().Add(-maxGap())
	for i, r := range recs {
		if !r.at.IsZero() && !r.at.Before(cutoff) {
			return recs[i:]
		}
	}
	return nil
}
