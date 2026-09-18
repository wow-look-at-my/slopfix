// staleness.go bounds how long an observation keeps a subject shut. A verdict
// goes out of date, so evidence ages out and the subject re-opens by itself.
package busypoll

import "time"

// recentRecords returns the records written inside the window. Records arrive
// in order, so it cuts a prefix, and an undated record keeps the position its
// neighbours give it. A transcript with no usable timestamp is returned whole:
// an unreadable clock must not turn this rule into a block.
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
