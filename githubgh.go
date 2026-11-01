// githubgh.go reads the GitHub API through the gh CLI when no token is in the environment.

package slopfix

import (
	"bufio"
	"bytes"
	"os/exec"
	"strconv"
	"strings"
)

// ghAPI runs `gh api --include path` and answers its stdout. A test replaces it.
var ghAPI = func(path string) ([]byte, error) {
	out, err := exec.Command("gh", "api", "--include", strings.TrimPrefix(path, "/")).Output()
	if len(out) > 0 {
		return out, nil
	}
	return nil, err
}

// githubGetViaGH answers path through gh, with the user's own credentials.
func githubGetViaGH(path string) (status int, body []byte, ok bool) {
	out, err := ghAPI(path)
	if err != nil || len(out) == 0 {
		return 0, nil, false
	}
	return parseIncluded(out)
}

// parseIncluded splits `gh api --include` output into the status of its first line and the body
// after the blank line.
func parseIncluded(out []byte) (int, []byte, bool) {
	reader := bufio.NewReader(bytes.NewReader(out))
	first, err := reader.ReadString('\n')
	if err != nil {
		return 0, nil, false
	}
	fields := strings.Fields(first)
	if len(fields) < 2 || !strings.HasPrefix(fields[0], "HTTP/") {
		return 0, nil, false
	}
	status, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, nil, false
	}
	for {
		line, err := reader.ReadString('\n')
		if strings.TrimRight(line, "\r\n") == "" {
			break
		}
		if err != nil {
			return status, nil, true
		}
	}
	var body bytes.Buffer
	_, _ = body.ReadFrom(reader)
	return status, body.Bytes(), true
}
