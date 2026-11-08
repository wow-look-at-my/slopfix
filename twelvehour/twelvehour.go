// Package twelvehour rewrites 24-hour clock times in displayed text to 12-hour
// times with am or pm. A UTC time also moves to the local zone.
package twelvehour

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	stamp    = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(?::\d{2})?(?:\.\d+)?Z\b`)
	clock    = regexp.MustCompile(`\b([01]?\d|2[0-3]):([0-5]\d)(?::[0-5]\d)?(Z\b)?`)
	code     = regexp.MustCompile("`[^`\n]*`|\\]\\([^)\n]*\\)|https?://[^\\s)]+")
	meridiem = regexp.MustCompile(`(?i)^\s*[ap]\.?m\b`)

	durationAfter  = regexp.MustCompile(`(?i)^\s*(?:mins?|minutes?|hrs?|hours?|long|elapsed)\b`)
	durationBefore = regexp.MustCompile(`(?i)\b(?:took|lasted|elapsed|runtime|duration)\s*:?\s*$`)
)

// Rewrite answers text with every 24-hour time written for a 12-hour reader.
func Rewrite(text string, now time.Time) string {
	out, _ := rewriteFrom(text, now, false)
	return out
}

// rewriteFrom rewrites text that starts inside a fence when inFence is true,
// and answers whether the text ends inside one.
func rewriteFrom(text string, now time.Time, inFence bool) (string, bool) {
	lines := strings.SplitAfter(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			lines[i] = rewriteLine(line, now)
		}
	}
	return strings.Join(lines, ""), inFence
}

// rewriteLine rewrites the stretches between the code spans of one line.
func rewriteLine(line string, now time.Time) string {
	var out strings.Builder
	at := 0
	for _, span := range code.FindAllStringIndex(line, -1) {
		out.WriteString(rewriteProse(line[at:span[0]], now))
		out.WriteString(line[span[0]:span[1]])
		at = span[1]
	}
	out.WriteString(rewriteProse(line[at:], now))
	return out.String()
}

func rewriteProse(text string, now time.Time) string {
	text = stamp.ReplaceAllStringFunc(text, func(s string) string {
		parsed, err := time.Parse(time.RFC3339Nano, strings.Replace(s, " ", "T", 1))
		if err != nil {
			return s
		}
		return parsed.In(now.Location()).Format("Jan 2 3:04 pm MST")
	})
	var out strings.Builder
	at := 0
	for _, m := range clock.FindAllStringSubmatchIndex(text, -1) {
		replacement, ok := convert(text, m, now)
		if !ok {
			continue
		}
		out.WriteString(text[at:m[0]])
		out.WriteString(replacement)
		at = m[1]
	}
	out.WriteString(text[at:])
	return out.String()
}

// isDuration reports whether the words around a match make it a length of time.
func isDuration(before, after string) bool {
	return durationAfter.MatchString(after) || durationBefore.MatchString(before)
}

func nearestMeridiem(hour, minute int, now time.Time) string {
	const day = 24 * 60
	clockNow := now.Hour()*60 + now.Minute()
	distance := func(target int) int {
		gap := (target - clockNow + day) % day
		return min(gap, day-gap)
	}
	morning := hour*60 + minute
	if distance(morning) < distance(morning+12*60) {
		return "am"
	}
	return "pm"
}

// convert answers the 12-hour spelling of one match.
func convert(text string, m []int, now time.Time) (string, bool) {
	if m[0] > 0 && strings.ContainsRune(".:/-#_", rune(text[m[0]-1])) {
		return "", false
	}
	if m[1] < len(text) && strings.ContainsRune(":/-_", rune(text[m[1]])) {
		return "", false
	}
	if meridiem.MatchString(text[m[1]:]) {
		return "", false
	}
	hourText := text[m[2]:m[3]]
	hour, err := strconv.Atoi(hourText)
	if err != nil {
		return "", false
	}
	minute := text[m[4]:m[5]]
	if m[6] >= 0 {
		minutes, err := strconv.Atoi(minute)
		if err != nil {
			return "", false
		}
		utc := time.Date(now.Year(), now.Month(), now.Day(), hour, minutes, 0, 0, time.UTC)
		return utc.In(now.Location()).Format("3:04 pm MST"), true
	}
	if isDuration(text[:m[0]], text[m[1]:]) {
		return "", false
	}
	if hour >= 1 && hour <= 11 && !strings.HasPrefix(hourText, "0") {
		minutes, err := strconv.Atoi(minute)
		if err != nil {
			return "", false
		}
		return fmt.Sprintf("%d:%s %s", hour, minute, nearestMeridiem(hour, minutes, now)), true
	}
	switch {
	case hour == 0:
		return fmt.Sprintf("12:%s am", minute), true
	case hour < 12:
		return fmt.Sprintf("%d:%s am", hour, minute), true
	case hour == 12:
		return fmt.Sprintf("12:%s pm", minute), true
	}
	return fmt.Sprintf("%d:%s pm", hour-12, minute), true
}
