package slopfix

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

// fixtureAPI is the GitHub API a rule case runs against.
var fixtureAPI struct {
	once  sync.Once
	mu    sync.Mutex
	table map[string]string
	err   error
}

// serveFixtureRefs adds a case's answers to the fixture API. The table maps
// "owner/repo" to the default branch, and "owner/repo@ref" to "branch" or
// "tag". A repository the table omits defaults to master, and a ref it omits
// is a tag.
func serveFixtureRefs(table map[string]string) error {
	fixtureAPI.once.Do(func() {
		fixtureAPI.table = map[string]string{}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			fixtureAPI.err = err
			return
		}
		go http.Serve(listener, http.HandlerFunc(answerFixtureRef))
		fixtureAPI.err = os.Setenv("GITHUB_API_URL", "http://"+listener.Addr().String())
	})
	if fixtureAPI.err != nil {
		return fixtureAPI.err
	}
	fixtureAPI.mu.Lock()
	defer fixtureAPI.mu.Unlock()
	for key, value := range table {
		fixtureAPI.table[key] = value
	}
	return nil
}

// answerFixtureRef serves both endpoints GitHubRefs reads.
func answerFixtureRef(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.EscapedPath(), "/repos/")
	owner, rest, _ := strings.Cut(path, "/")
	name, rest, _ := strings.Cut(rest, "/")
	repo := owner + "/" + name
	fixtureAPI.mu.Lock()
	defer fixtureAPI.mu.Unlock()
	if rest == "" {
		def, ok := fixtureAPI.table[repo]
		if !ok {
			def = "master"
		}
		json.NewEncoder(w).Encode(map[string]string{"default_branch": def})
		return
	}
	space, ref, ok := strings.Cut(strings.TrimPrefix(rest, "git/ref/"), "/")
	if !ok || !strings.HasPrefix(rest, "git/ref/") {
		http.NotFound(w, r)
		return
	}
	ref, err := url.PathUnescape(ref)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	kind, named := fixtureAPI.table[repo+"@"+ref]
	if !named {
		kind = "tag"
	}
	if (space == "heads" && kind == "branch") || (space == "tags" && kind == "tag") {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.NotFound(w, r)
}
