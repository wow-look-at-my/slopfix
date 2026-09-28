// no-work-loss: a PreToolUse hook that refuses the ways a session loses
// authorship of the working tree.
//
//   - Destruction: a command that would destroy content existing only in the
//     working tree. A modified-but-uncommitted or untracked file is in no git
//     object, so losing it is unrecoverable; committed work is reachable from
//     the reflog and is deliberately NOT protected.
//   - Provenance: a change to file content that does not go through Write, Edit
//     or NotebookEdit. Bash runs things -- git, builds, tests, validation,
//     search -- and does not author files.
//
// Both are asked of the same parsed command, which is why they share a plugin:
// the shell walk, the wrapper stripping and the path resolution are the same
// machinery.
//
// # Destruction
//
// The invariant: content that exists only in the working tree must never be lost by a command the agent runs. Committed work stays in the reflog. A history rewrite is thus out of scope.
//
// The hook preserves, then allows. It commits the at-risk paths onto the CURRENT BRANCH, where the log and the next push already look. A hidden ref is one nobody reviews.
//
//   - Every step uses a throwaway `GIT_INDEX_FILE`, seeded by `git read-tree HEAD`. The real index and the working tree never move.
//   - `stagedTree` commits the index entries first. `git add --force` then commits the disk content. A tree equal to its parent is dropped. No commit is therefore empty.
//   - `git update-ref HEAD` advances the branch. `git reset -q <commit> -- <paths>` refreshes the real index for those paths. Git hooks are off during preservation.
//   - A best-effort `git push --no-verify origin HEAD` follows. A push failure still allows. The report names where the content sits.
//   - A failed `add`, `write-tree`, `commit-tree` or `update-ref` denies. Preservation that did not happen must never read as success.
//
// A stash entry and a ref-destroying command still deny outright. A ref under `refs/no-work-loss/` is the only copy of its content. Deleting or force-pushing it is always refused.
//
// The incident: `git checkout master` carries a dirty tree across. `git reset --hard origin/master` then destroys it with no reflog entry. A guard on `reset --hard` alone misses the setup step.
//
// Each verb is checked against the classes it reaches. A single dirty bit gives false positives that get a guard uninstalled.
//
//	| Command | tracked edits | untracked | ignored | stash |
//	|---|---|---|---|---|
//	| `reset --hard` | destroys | spares | spares | spares |
//	| `clean -fd` | spares | destroys | spares | spares |
//	| `clean -fdx` | spares | destroys | destroys | spares |
//	| `stash drop` | spares | spares | spares | destroys |
//	| `checkout <ref>` | destroys | spares | spares | spares |
//
// A submodule at a commit other than its gitlink is not a tracked edit. That commit stays in the history of the submodule. The probe reads `status --porcelain=v2` and drops a submodule entry that has no modified and no untracked content.
//
// A ref-destroying verb asks whether the content exists anywhere else:
//
//	| Verb | What must survive | How it is answered |
//	|---|---|---|
//	| `branch -D` / `-M`, `update-ref -d`, `push --delete` | the tip | another ref contains it |
//	| `push --force` / `+refspec` | the remote-tracking tip | an ancestor of the push, or in another ref |
//	| `filter-branch` | all of HEAD | `rev-list --count HEAD --not --remotes` is `0` |
//	| `reflog expire` / `delete` | nothing reflog-only | `fsck --unreachable --no-reflogs` finds no commit |
//	| `worktree remove --force` | its edits | its `status --porcelain` is empty |
//
//   - Containment uses `for-each-ref --contains`. The `--exclude` flag with `--branches` takes the name without `refs/heads/`. A full refname thus excludes nothing.
//   - `refs/remotes/<remote>/HEAD` aliases the overwritten branch. The check filters it out.
//   - `push --mirror` is always refused. A push with no local remote-tracking ref denies and names `git fetch`.
//
// Allowed on purpose: unpushed commits on a clean tree, `checkout -b`, `switch -c`, `stash push`, `commit`, `add`, `restore --staged`, unknown verbs, appends, and anything outside a repository.
//
// # Detection
//
//   - Chains, pipes, subshells, bodies and command substitutions are each evaluated.
//   - A `cd` carries forward across `&&`, `||`, `;` and brace blocks. A pipe stage, a subshell and a conditional body get a copy. `git -C` and `--work-tree` apply too.
//   - Wrappers resolve to their program, with value-taking flags understood: `env`, `sudo`, `doas`, `command`, `builtin`, `exec`, `nohup`, `nice`, `ionice`, `setsid`, `stdbuf`, `timeout`, `xargs`. A leading `\` and an absolute path.
//   - Short flags unbundle. A `--flag=value` registers as `--flag`.
//   - Git aliases resolve from `git config`, including `!shell` aliases. Self-reference stops at depth `3`.
//
// Ambiguity denies. That covers an unparseable command with a destructive verb, an operand that is not static (`rm $TARGET`, `cd -`), and a relocated `GIT_DIR`. A script FILE that the walk follows is judged on its static paths only.
//
// A device target such as `/dev/null`, and a descriptor other than stdout, cannot empty a file. The provenance half still judges every descriptor. It thus refuses `echo x 2> tracked.go`.
//
// A destructive verb fails closed on a panic, a git error or the `3`-second git timeout. Everything else fails open. A killed binary lets the command run, because a hook cannot make itself mandatory. `clean-bash` rewrites `rm`. This hook sees the original command. Where both fire, the deny wins.
//
// # Provenance
//
// Any change to file content in the working tree goes through Write, Edit or NotebookEdit. The verdict follows from the shape of the write:
//
//	| Shape | Example | Denies when |
//	|---|---|---|
//	| named path | `sed -i x.go`, `> x.go`, `cp a x.go` | the path is in a guarded root and not under a build directory |
//	| whole directory | `patch`, `tar -x`, `git apply` | the directory holds or sits in a guarded root |
//	| opaque | `node -e`, an `xargs`-fed `sed -i`, a GitHub API commit | always |
//
//   - Routes include editors, `busybox` applets, every file redirect, `tee`, `dd of=`, `truncate -s`, `sponge`, `xxd -r`, `sort -o`, `split`, compressors without `-c`, `zip`, `docker cp`, `yq -i` and `ln`.
//   - `cp`, `mv`, `install`, `rsync` and `scp` test the source side. A plain `mv old.go new.go` thus passes.
//   - `git apply`, `git am`, `rebase` and `revert` are refused. `git merge`, `git pull` and `git cherry-pick` pass, because git already holds what they land.
//   - Indirection is followed: `sh -c`, aliases, script files, `find -exec`, functions and wrappers.
//   - A long in-place flag (`--in-place`, `--write`) counts for any program. `allowedFormatter` lists. The tools that rewrite by design. `jq` has an empty flag set on purpose.
//   - A subagent spawn with a tool grant or a permissive `permissionMode` is refused. The live settings files are refused to every tool.
//   - The session scratchpad is the one temporary directory that does not deny.
//
// A Write over a path that git holds and the disk does not is refused, however the path was emptied. A deletion committed on the branch, or in the last `10` commits, holds the path too. The hook restores the file with `git restore --worktree` first, from the index, HEAD or the parent of the deleting commit. A file in the recycle bin comes back the same way, through `recycler restore` with the newest item's ID. No route lets Write replace a file that existed. The refusal names the Edit tool. It prices the evasion in Claude tokens from `transcript_path`. The price is every tool call that names the file since its last Read or Edit, plus this Write. The changed lines are subtracted. `echo x > new-file` is refused too, because creating a file is what Write is for.
package noworkloss

import (
	"encoding/json"
	"io"
	"os"
	"strings"
)

// IDDestruction names the refusal of a command destroying working-tree content.
const IDDestruction = "noworkloss/destruction"

// IDProvenance names the refusal of a change routed around the edit tools.
const IDProvenance = "noworkloss/provenance"

// IDWriteTool names the refusal of a Write over a path that already holds content.
const IDWriteTool = "noworkloss/write-tool"

type hookInput struct {
	HookEventName  string          `json:"hook_event_name"`
	ToolName       string          `json:"tool_name"`
	Cwd            string          `json:"cwd"`
	ToolInput      json.RawMessage `json:"tool_input"`
	ToolUseID      string          `json:"tool_use_id"`
	TranscriptPath string          `json:"transcript_path"`
}

// Result is what an invocation emits. A refusal rides stdout as a deny
// payload, which is how a PreToolUse hook stops a call before it runs.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// The CLI rejects a payload whose hookEventName is not the event it
// dispatched, so the shape follows the event rather than the verdict.
type preToolUseResponse struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// preToolUseNotice carries no permissionDecision: a preservation leaves the
// permission flow untouched and only says where the content went.
type preToolUseNotice struct {
	HookSpecificOutput struct {
		HookEventName string `json:"hookEventName"`
	} `json:"hookSpecificOutput"`
	SystemMessage string `json:"systemMessage"`
}

// Run reads a PreToolUse payload from r and decides the call it describes.
func Run(r io.Reader) Result {
	raw, err := io.ReadAll(r)
	if err != nil {
		// Reading the payload failed, so nothing is known about the call. A hook that knows nothing stays out of the way.
		return Result{}
	}
	reason, notices := decide(raw)
	if reason != "" {
		return Result{Stdout: denyPayload(reason)}
	}
	// A preservation moved content into a ref, which must never happen silently.
	if len(notices) > 0 {
		return Result{Stdout: noticePayload(notices)}
	}
	return Result{}
}

// evaluateLoss runs the destruction analysis under a recover, failing OPEN on a panic.
func evaluateLoss(command, cwd string) (reason string, notices []string) {
	// A cheap byte scan leads: the overwhelming majority of Bash calls name no
	// verb that can delete anything, and those must not pay for a parse.
	if command == "" || !mayDestroy(command) {
		return "", nil
	}
	defer func() {
		if r := recover(); r != nil {
			reason, notices = "", nil
			if verb, ok := destructiveKeyword(command); ok {
				reason = internalErrorReason(verb)
			}
		}
	}()
	return analyze(command, cwd)
}

func denyPayload(reason string) string {
	var resp preToolUseResponse
	resp.HookSpecificOutput.HookEventName = "PreToolUse"
	resp.HookSpecificOutput.PermissionDecision = "deny"
	resp.HookSpecificOutput.PermissionDecisionReason = reason
	out, err := json.Marshal(resp)
	if err != nil {
		return ""
	}
	return string(out)
}

// emitDeny writes a denial to stdout, which is what the raw-byte tests drive.
func emitDeny(reason string) {
	if out := denyPayload(reason); out != "" {
		os.Stdout.WriteString(out)
	}
}

func noticePayload(notices []string) string {
	var resp preToolUseNotice
	resp.HookSpecificOutput.HookEventName = "PreToolUse"
	resp.SystemMessage = strings.Join(notices, "\n")
	out, err := json.Marshal(resp)
	if err != nil {
		return ""
	}
	return string(out)
}
