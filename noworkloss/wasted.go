package noworkloss

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	tokenizer "github.com/wow-look-at-my/go-tokenizer"
)

// The price of a refused Write over a file that existed, read from the
// session transcript and counted in Claude tokens.

// writeAttempt is the refused Write call as the payload carries it.
type writeAttempt struct {
	input      json.RawMessage
	content    string
	id         string
	transcript string
}

const wasteMethod = "method: every tool call since the last Read or Edit of this file that names it, or runs git add, rm or commit after one did, plus this Write, minus an Edit of the lines that differ from the restored content"

// wasted is the counter line of the refusal. A count that cannot be made says
// why, and the refusal stands either way.
func (w writeAttempt) wasted(path string, held heldPath) string {
	n, err := w.wastedTokens(path, held)
	if err != nil {
		return "Tokens wasted on this evasion compared to one Edit: unavailable (" + err.Error() + ")."
	}
	return "Tokens wasted on this evasion compared to one Edit: " + strconv.Itoa(n) + " (" + wasteMethod + ")."
}

func (w writeAttempt) wastedTokens(path string, held heldPath) (int, error) {
	if w.transcript == "" {
		return 0, errors.New("the payload names no transcript")
	}
	calls, err := readToolCalls(w.transcript)
	if err != nil {
		return 0, fmt.Errorf("the transcript cannot be read: %w", err)
	}
	before, err := held.content()
	if err != nil {
		return 0, fmt.Errorf("git cannot show the restored content: %w", err)
	}
	est, err := tokenizer.NewAnthropicEstimator(tokenizer.FamilyClaude5)
	if err != nil {
		return 0, fmt.Errorf("the tokenizer did not load: %w", err)
	}
	spent := []string{string(w.input)}
	for _, c := range evasion(calls, path, w.id) {
		spent = append(spent, string(c.input))
	}
	removed, added := changedLines(before, w.content)
	total := 0
	for _, text := range spent {
		n, err := est.CountTokens(text)
		if err != nil {
			return 0, err
		}
		total += n
	}
	for _, text := range []string{removed, added} {
		n, err := est.CountTokens(text)
		if err != nil {
			return 0, err
		}
		total -= n
	}
	return total, nil
}

// toolCall is one tool_use block of an assistant record.
type toolCall struct {
	id, name string
	input    json.RawMessage
}

func readToolCalls(path string) ([]toolCall, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var calls []toolCall
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 256<<20)
	for sc.Scan() {
		var rec struct {
			Type    string `json:"type"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(sc.Bytes(), &rec) != nil || rec.Type != "assistant" {
			continue
		}
		var blocks []struct {
			Type  string          `json:"type"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}
		// An assistant record whose content is a plain string holds no tool call.
		if json.Unmarshal(rec.Message.Content, &blocks) != nil {
			continue
		}
		for _, b := range blocks {
			if b.Type == "tool_use" {
				calls = append(calls, toolCall{id: b.ID, name: b.Name, input: b.Input})
			}
		}
	}
	return calls, sc.Err()
}

// evasion is the run of calls spent on the path since its content was last
// intact. A Read or Edit of the file marks it intact. The run opens at the
// first later call that names the file; a git add, rm or commit counts once
// it is open, because `git add -A` names no path. The call being decided is
// priced from the payload, so its transcript copy is skipped.
func evasion(calls []toolCall, path, skipID string) []toolCall {
	base := filepath.Base(path)
	var run []toolCall
	open := false
	for _, c := range calls {
		if skipID != "" && c.id == skipID {
			continue
		}
		var in struct {
			FilePath string `json:"file_path"`
			Command  string `json:"command"`
		}
		_ = json.Unmarshal(c.input, &in)
		target := in.FilePath != "" && samePath(in.FilePath, path)
		switch {
		case target && (c.name == "Read" || c.name == "Edit" || c.name == "MultiEdit"):
			run, open = nil, false
		case target, in.Command != "" && strings.Contains(in.Command, base):
			run, open = append(run, c), true
		case open && gitStep(in.Command):
			run = append(run, c)
		}
	}
	return run
}

func samePath(p, path string) bool {
	p = filepath.Clean(p)
	return p == path || p == physicalPath(path) || physicalPath(p) == physicalPath(path)
}

func gitStep(command string) bool {
	for _, verb := range []string{"git add", "git rm", "git commit"} {
		if strings.Contains(command, verb) {
			return true
		}
	}
	return false
}

// changedLines answers the lines an Edit would carry: the removed ones and the
// added ones, each joined.
func changedLines(before, after string) (removed, added string) {
	a, b := strings.SplitAfter(before, "\n"), strings.SplitAfter(after, "\n")
	var rm, add strings.Builder
	for _, op := range difflib.NewMatcherWithJunk(a, b, false, nil).GetOpCodes() {
		if op.Tag == 'e' {
			continue
		}
		rm.WriteString(strings.Join(a[op.I1:op.I2], ""))
		add.WriteString(strings.Join(b[op.J1:op.J2], ""))
	}
	return rm.String(), add.String()
}
