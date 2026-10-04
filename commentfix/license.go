package commentfix

import "regexp"

// licenseNotice matches a copyright line or an SPDX tag.
var licenseNotice = regexp.MustCompile(`(?i)\bcopyright\b|\(c\)\s*(?:19|20)[0-9]{2}|SPDX-License-Identifier`)

// isLicenseNotice reports a block that carries a license notice. No rule
// measures it and no repair touches it.
func isLicenseNotice(text []string) bool {
	for _, line := range text {
		if licenseNotice.MatchString(line) {
			return true
		}
	}
	return false
}

// withoutLicenseNotices drops every block that carries a license notice.
func withoutLicenseNotices(in []block) []block {
	out := make([]block, 0, len(in))
	for _, b := range in {
		if isLicenseNotice(b.text) {
			continue
		}
		out = append(out, b)
	}
	return out
}
