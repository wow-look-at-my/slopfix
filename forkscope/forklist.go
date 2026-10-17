package forkscope

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/tidwall/jsonc"
	"github.com/wow-look-at-my/go-containers/set"
)

// listedUpstream answers the upstream URL that the fork list at url gives repo,
// an OWNER/NAME pair. A missing list, and a repository the list does not name,
// answer an empty URL.
func listedUpstream(repo, url string) (string, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return "", fmt.Errorf("fork scope: the repository is %q, which is not OWNER/NAME", repo)
	}
	status, body, err := getList(url)
	if err != nil {
		return "", err
	}
	if status == http.StatusNotFound {
		return "", nil
	}
	return UpstreamFor(body, repo, url)
}

// listAttempts is how many times getList asks for the list before it reports the failure.
const listAttempts = 3

// listRetryWait is the pause before each later attempt. A test sets it to zero.
var listRetryWait = func(attempt int) { time.Sleep(time.Duration(attempt) * time.Second) }

// getList fetches the fork list. A transport error and a server error are a
// blip of the host, so it asks again before it fails the run.
func getList(url string) (int, string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	var last error
	for attempt := 1; attempt <= listAttempts; attempt++ {
		if attempt > 1 {
			listRetryWait(attempt - 1)
		}
		resp, err := client.Get(url)
		if err != nil {
			last = fmt.Errorf("fork scope: GET %s: %w", url, err)
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if err != nil {
			last = fmt.Errorf("fork scope: GET %s: %w", url, err)
			continue
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			last = fmt.Errorf("fork scope: GET %s: %s", url, resp.Status)
			continue
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			return 0, "", fmt.Errorf("fork scope: GET %s: %s", url, resp.Status)
		}
		return resp.StatusCode, string(body), nil
	}
	return 0, "", last
}

// listName is the shape forklist.schema.json gives a key of the fork list.
var listName = regexp.MustCompile(`^[^/\s]+/[^/\s]+$`)

// UpstreamFor reads a fork list. A JSON object, comments allowed, that maps
// each fork's OWNER/NAME to its upstream URL, the shape forklist.schema.json
// states. A list of any other shape is an error.
func UpstreamFor(list, repo, source string) (string, error) {
	var raw any
	if err := json.Unmarshal(jsonc.ToJSON([]byte(list)), &raw); err != nil {
		return "", fmt.Errorf("fork scope: %s is not a fork list: %w", source, err)
	}
	forks, ok := raw.(map[string]any)
	if !ok {
		return "", fmt.Errorf("fork scope: %s is not a fork list: it is not an object", source)
	}
	found := ""
	for name, value := range forks {
		if !listName.MatchString(name) {
			return "", fmt.Errorf("fork scope: %s is not a fork list: %q is not OWNER/NAME", source, name)
		}
		url, ok := value.(string)
		if !ok || url == "" {
			return "", fmt.Errorf("fork scope: %s is not a fork list: %q maps to no URL", source, name)
		}
		if strings.EqualFold(name, repo) {
			found = url
		}
	}
	return found, nil
}

// upstreamTags answers the commit each tag of upstream names. An upstream with
// no tags answers none.
func upstreamTags(upstream string) ([]string, error) {
	out, err := gitIn(".", "ls-remote", "--tags", upstream)
	if err != nil {
		return nil, fmt.Errorf("fork scope: list the tags of %s: %w", upstream, err)
	}
	direct := map[string]string{}
	peeled := map[string]string{}
	for line := range strings.SplitSeq(out, "\n") {
		sha, ref, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		if name, isPeel := strings.CutSuffix(ref, "^{}"); isPeel {
			peeled[name] = sha
		} else {
			direct[ref] = sha
		}
	}
	commits := set.New[string]()
	for ref, sha := range direct {
		if commit, ok := peeled[ref]; ok {
			sha = commit
		}
		commits.Add(sha)
	}
	return commits.Values(), nil
}
