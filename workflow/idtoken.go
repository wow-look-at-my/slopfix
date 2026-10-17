package workflow

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
)

// IDIDToken is a job whose permissions do not grant id-token: write.
const IDIDToken = "yaml/id-token"

// The org's actions mint a GitHub OIDC token to reach buildhost and secret-server. A job without
// id-token: write fails the first such step, so every job grants it.
const idTokenLine = "id-token: write"

// idTokenBlock is a permissions mapping that some job runs under and that does not grant id-token: write.
type idTokenBlock struct {
	block *yaml.Node
	// level is the id-token value the block sets, or nil when it sets none.
	level *yaml.Node
}

// idTokenBlocks answers each permissions mapping that leaves a job without id-token: write, once each.
// A job with no block anywhere runs under the repository default, and a block written for it would set
// every other permission to none, so no repair can say what it holds. read-all names no permission
// a line could raise.
func idTokenBlocks(content string) []idTokenBlock {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	root := rootOf(&doc)
	jobs := mappingValue(root, "jobs")
	if jobs == nil || jobs.Kind != yaml.MappingNode {
		return nil
	}
	workflowBlock := mappingValue(root, "permissions")
	seen := map[*yaml.Node]bool{}
	var out []idTokenBlock
	for i := 1; i < len(jobs.Content); i += 2 {
		block := mappingValue(jobs.Content[i], "permissions")
		if block == nil {
			block = workflowBlock
		}
		if block == nil || block.Kind != yaml.MappingNode || seen[block] {
			continue
		}
		seen[block] = true
		level := mappingValue(block, "id-token")
		if level != nil && level.Value == "write" {
			continue
		}
		// A flow mapping over several lines has no single line the repair can write.
		if block.Style&yaml.FlowStyle != 0 && len(block.Content) > 0 && block.Content[len(block.Content)-1].Line != block.Line {
			continue
		}
		out = append(out, idTokenBlock{block: block, level: level})
	}
	return out
}

// idTokens reports each permissions block that leaves a job without id-token: write.
func idTokens(content string) []ste.Finding {
	var out []ste.Finding
	for _, b := range idTokenBlocks(content) {
		line := b.block.Line
		if b.level != nil {
			line = b.level.Line
		}
		out = append(out, ste.Finding{
			Line: line,
			ID:   IDIDToken,
			Rule: "a job without id-token: write cannot mint the OIDC token the org's actions reach buildhost and secret-server with",
			Fix:  "Grant `" + idTokenLine + "` in the permissions block the job runs under. `slopfix fix` does this.",
		})
	}
	return out
}

// grantIDToken writes id-token: write into each block idTokenBlocks names. A value that is there is
// raised in place. A block mapping takes a line before its first entry, at that entry's indent. A flow
// mapping on one line takes the entry after its brace.
func grantIDToken(content string) []edit.Edit {
	fileLines := lines(content)
	var out []edit.Edit
	for _, b := range idTokenBlocks(content) {
		switch {
		case b.level != nil:
			row := b.level.Line - 1
			text := fileLines[row]
			at := b.level.Column - 1
			out = append(out, rewrite(content, row, row, []string{text[:at] + "write" + text[at+len(b.level.Value):]}))
		case b.block.Style&yaml.FlowStyle != 0:
			row := b.block.Line - 1
			text := fileLines[row]
			at := b.block.Column - 1
			if at >= len(text) || text[at] != '{' {
				continue
			}
			entry := idTokenLine
			if strings.TrimSpace(text[at+1:])[0] != '}' {
				entry += ", "
			}
			out = append(out, rewrite(content, row, row, []string{text[:at+1] + entry + text[at+1:]}))
		default:
			first := b.block.Content[0]
			row := first.Line - 1
			indent := fileLines[row][:first.Column-1]
			out = append(out, rewrite(content, row, row, []string{indent + idTokenLine, fileLines[row]}))
		}
	}
	return out
}
