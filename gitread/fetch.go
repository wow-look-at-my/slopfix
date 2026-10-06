package gitread

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchRef resolves a ref on a remote, downloads the objects behind it, and
// answers the object and a repository that serves them. A local remote is read
// where it sits; a network remote is fetched over smart HTTP.
func FetchRef(remote, ref string, seed *Repo) (OID, *Repo, error) {
	if local, ok := OpenRemote(remote); ok {
		oid, err := resolveOn(local, ref)
		if err != nil {
			return OID{}, nil, err
		}
		return oid, local, nil
	}
	refs, caps, err := httpRefsAndCaps(remote)
	if err != nil {
		return OID{}, nil, err
	}
	oid, err := resolveIn(refs, caps, ref)
	if err != nil {
		return OID{}, nil, err
	}
	pack, err := fetchPack(remote, caps, oid)
	if err != nil {
		return OID{}, nil, err
	}
	objs, err := parsePack(pack, seed)
	if err != nil {
		return OID{}, nil, err
	}
	return oid, &Repo{objs: objs}, nil
}

// resolveOn answers a ref in a local repository, with HEAD special-cased.
func resolveOn(r *Repo, ref string) (OID, error) {
	if ref == "HEAD" {
		return r.Head()
	}
	return r.Resolve(ref)
}

// resolveIn answers a ref from a remote's advertised refs, following the
// symref capability for HEAD.
func resolveIn(refs map[string]OID, caps, ref string) (OID, error) {
	if ref == "HEAD" {
		if target := symrefTarget(caps, "HEAD"); target != "" {
			if oid, ok := refs[target]; ok {
				return oid, nil
			}
		}
		if oid, ok := refs["HEAD"]; ok {
			return oid, nil
		}
	}
	if oid, ok := refs[ref]; ok {
		return oid, nil
	}
	return OID{}, fmt.Errorf("gitread: %s names no object on this remote", ref)
}

// symrefTarget reads the target of a symbolic ref from a capability line.
func symrefTarget(caps, ref string) string {
	for _, cap := range strings.Fields(caps) {
		value, ok := strings.CutPrefix(cap, "symref=")
		if !ok {
			continue
		}
		name, target, ok := strings.Cut(value, ":")
		if ok && name == ref {
			return target
		}
	}
	return ""
}

// fetchPack runs an upload-pack request for one object and answers its pack.
func fetchPack(remote string, caps string, want OID) ([]byte, error) {
	base := strings.TrimSuffix(strings.TrimRight(remote, "/"), ".git")
	target := base + ".git/git-upload-pack"
	var body bytes.Buffer
	wanted := mapCaps(caps, "multi_ack_detailed", "side-band-64k", "ofs-delta", "filter")
	writePkt(&body, "want "+want.String()+" "+wanted+"\n")
	if strings.Contains(caps, "filter") {
		writePkt(&body, "filter blob:none\n")
	}
	body.WriteString("0000")
	writePkt(&body, "done\n")

	req, err := http.NewRequest(http.MethodPost, target, &body)
	if err != nil {
		return nil, fmt.Errorf("gitread: POST %s: %w", target, err)
	}
	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	req.Header.Set("Accept", "application/x-git-upload-pack-result")
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitread: POST %s: %w", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitread: POST %s: %s", target, resp.Status)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gitread: read %s: %w", target, err)
	}
	return demuxPack(raw)
}

// mapCaps keeps the capabilities the remote advertised.
func mapCaps(caps string, names ...string) string {
	have := map[string]bool{}
	for _, cap := range strings.Fields(caps) {
		name, _, _ := strings.Cut(cap, "=")
		have[name] = true
	}
	var out []string
	for _, name := range names {
		if have[name] {
			out = append(out, name)
		}
	}
	return strings.Join(out, " ")
}

// writePkt writes one pkt-line.
func writePkt(w *bytes.Buffer, payload string) {
	fmt.Fprintf(w, "%04x%s", len(payload)+4, payload)
}

// demuxPack reads the side-band packets of an upload-pack answer and answers
// the packfile they carry.
func demuxPack(raw []byte) ([]byte, error) {
	var pack bytes.Buffer
	rest := raw
	for len(rest) >= 4 {
		size := pktSize(rest[:4])
		if size == 0 {
			rest = rest[4:]
			continue
		}
		if size < 4 || size > len(rest) {
			return nil, fmt.Errorf("gitread: a pkt-line is malformed")
		}
		payload := rest[4:size]
		rest = rest[size:]
		if len(payload) == 0 {
			continue
		}
		switch payload[0] {
		case 1:
			pack.Write(payload[1:])
		case 2:
			// Progress goes nowhere.
		case 3:
			return nil, fmt.Errorf("gitread: the remote refused the fetch: %s", strings.TrimSpace(string(payload[1:])))
		default:
		}
	}
	if pack.Len() == 0 {
		return nil, fmt.Errorf("gitread: the remote sent no packfile")
	}
	return pack.Bytes(), nil
}

// parsePack reads a packfile into memory, resolving every delta chain.
func parsePack(pack []byte, seed *Repo) (map[OID]object, error) {
	cr := &countReader{r: bytes.NewReader(pack)}
	br := bufio.NewReader(cr)
	header := make([]byte, 12)
	if _, err := io.ReadFull(br, header); err != nil {
		return nil, fmt.Errorf("gitread: read the pack header: %w", err)
	}
	if string(header[:4]) != "PACK" {
		return nil, fmt.Errorf("gitread: the fetched data is not a packfile")
	}
	count := int(header[8])<<24 | int(header[9])<<16 | int(header[10])<<8 | int(header[11])
	byOffset := map[int]object{}
	byOID := map[OID]object{}
	position := func() int { return int(cr.n) - br.Buffered() }
	for i := 0; i < count; i++ {
		off := position()
		typ, size, err := readObjHeader(br)
		if err != nil {
			return nil, fmt.Errorf("gitread: read a pack object: %w", err)
		}
		switch typ {
		case 1, 2, 3, 4:
			data, err := inflate(br, size)
			if err != nil {
				return nil, err
			}
			obj := object{typ: map[int]string{1: TypeCommit, 2: TypeTree, 3: TypeBlob, 4: TypeTag}[typ], data: data}
			storeObject(obj, off, byOffset, byOID)
		case 6:
			neg, err := readOfsDelta(br)
			if err != nil {
				return nil, err
			}
			delta, err := inflate(br, size)
			if err != nil {
				return nil, err
			}
			base, ok := byOffset[off-int(neg)]
			if !ok {
				return nil, fmt.Errorf("gitread: an ofs-delta names no base")
			}
			data, err := applyDelta(base.data, delta)
			if err != nil {
				return nil, err
			}
			storeObject(object{typ: base.typ, data: data}, off, byOffset, byOID)
		case 7:
			var baseOID OID
			if _, err := io.ReadFull(br, baseOID[:]); err != nil {
				return nil, err
			}
			delta, err := inflate(br, size)
			if err != nil {
				return nil, err
			}
			base, ok := byOID[baseOID]
			if !ok && seed != nil {
				if found, err := seed.object(baseOID); err == nil {
					base, ok = found, true
				}
			}
			if !ok {
				return nil, fmt.Errorf("gitread: a ref-delta names a base outside the pack")
			}
			data, err := applyDelta(base.data, delta)
			if err != nil {
				return nil, err
			}
			storeObject(object{typ: base.typ, data: data}, off, byOffset, byOID)
		default:
			return nil, fmt.Errorf("gitread: pack object type %d is unknown", typ)
		}
	}
	return byOID, nil
}

// storeObject records an object under its offset and its computed name.
func storeObject(obj object, off int, byOffset map[int]object, byOID map[OID]object) {
	byOffset[off] = obj
	byOID[objectOID(obj)] = obj
}

// objectOID answers the SHA-1 name of an object.
func objectOID(obj object) OID {
	h := sha1.New()
	fmt.Fprintf(h, "%s %d\x00", obj.typ, len(obj.data))
	h.Write(obj.data)
	var out OID
	copy(out[:], h.Sum(nil))
	return out
}

// countReader counts every byte read from the underlying reader.
type countReader struct {
	r io.Reader
	n int64
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// httpRefsAndCaps answers a remote's advertised refs and its capability line.
func httpRefsAndCaps(remote string) (map[string]OID, string, error) {
	base := strings.TrimSuffix(strings.TrimRight(remote, "/"), ".git")
	target := base + ".git/info/refs?service=git-upload-pack"
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(target)
	if err != nil {
		return nil, "", fmt.Errorf("gitread: GET %s: %w", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("gitread: GET %s: %s", target, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, "", fmt.Errorf("gitread: read %s: %w", target, err)
	}
	return parseRefAdvertisement(body)
}
