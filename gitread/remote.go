package gitread

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// AddAlternate makes this repository fall back to other for objects it lacks.
func (r *Repo) AddAlternate(other *Repo) {
	if other == nil || other == r {
		return
	}
	r.mu.Lock()
	r.extra = append(r.extra, other)
	r.mu.Unlock()
}

// OpenRemote opens a remote that is a local path or a file:// URL, or false
// when the remote speaks over the network.
func OpenRemote(remote string) (*Repo, bool) {
	path := remote
	if strings.HasPrefix(remote, "file://") {
		u, err := url.Parse(remote)
		if err != nil {
			return nil, false
		}
		path = u.Path
	} else if strings.Contains(remote, "://") || strings.Contains(remote, "@") {
		return nil, false
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	r, ok := discover(path)
	if !ok {
		return nil, false
	}
	return r, true
}

// Tags answers the commit each tag of a remote names, peeled.
func Tags(remote string) ([]OID, error) {
	if isLocalRemote(remote) {
		local, ok := OpenRemote(remote)
		if !ok {
			return nil, fmt.Errorf("gitread: %s is not a local repository", remote)
		}
		return localTagCommits(local)
	}
	return httpTagCommits(remote)
}

// localTagCommits answers the peeled commit of each tag in a local repository.
func localTagCommits(r *Repo) ([]OID, error) {
	tags := r.Refs("refs/tags/")
	out := make([]OID, 0, len(tags))
	for _, oid := range tags {
		if commit, err := r.PeelCommit(oid); err == nil {
			out = append(out, commit)
		}
	}
	return out, nil
}

// httpTagCommits reads the ref advertisement over smart HTTP and peels its tags.
func httpTagCommits(remote string) ([]OID, error) {
	refs, err := httpRefs(remote)
	if err != nil {
		return nil, err
	}
	peeled := map[string]OID{}
	for ref, oid := range refs {
		if base, isPeel := strings.CutSuffix(ref, "^{}"); isPeel {
			peeled[base] = oid
		}
	}
	var out []OID
	for ref, oid := range refs {
		if !strings.HasPrefix(ref, "refs/tags/") || strings.HasSuffix(ref, "^{}") {
			continue
		}
		if commit, ok := peeled[ref]; ok {
			oid = commit
		}
		out = append(out, oid)
	}
	return out, nil
}

// httpRefs answers the ref name to object name map an HTTP remote advertises.
func httpRefs(remote string) (map[string]OID, error) {
	base := strings.TrimSuffix(strings.TrimRight(remote, "/"), ".git")
	target := base + ".git/info/refs?service=git-upload-pack"
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(target)
	if err != nil {
		return nil, fmt.Errorf("gitread: GET %s: %w", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitread: GET %s: %s", target, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("gitread: read %s: %w", target, err)
	}
	refs, _, err := parseRefAdvertisement(body)
	return refs, err
}

// parseRefAdvertisement reads pkt-line refs from a smart HTTP advertisement,
// and answers the capability line the first ref carries.
func parseRefAdvertisement(body []byte) (map[string]OID, string, error) {
	out := map[string]OID{}
	caps := ""
	rest := body
	first := true
	for len(rest) >= 4 {
		size := pktSize(rest[:4])
		if size == 0 {
			rest = rest[4:]
			if first {
				first = false
			}
			continue
		}
		if size < 4 || size > len(rest) {
			break
		}
		line := rest[4:size]
		rest = rest[size:]
		if first {
			first = false
			// The service banner is not a ref.
			if bytes.HasPrefix(line, []byte("# service=")) {
				continue
			}
		}
		line = bytes.TrimRight(line, "\n")
		if nul := bytes.IndexByte(line, 0); nul >= 0 {
			if caps == "" {
				caps = string(line[nul+1:])
			}
			line = line[:nul]
		}
		fields := bytes.Fields(line)
		if len(fields) != 2 {
			continue
		}
		oid, err := ParseOID(string(fields[0]))
		if err != nil {
			continue
		}
		out[string(fields[1])] = oid
	}
	return out, caps, nil
}

// pktSize reads a pkt-line's hexadecimal length.
func pktSize(header []byte) int {
	value := 0
	for _, c := range header {
		n, ok := hexVal(c)
		if !ok {
			return 0
		}
		value = value<<4 | int(n)
	}
	return value
}
