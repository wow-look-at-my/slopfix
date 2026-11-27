# auto-allow

`auto-allow` approves read-only work and refuses a program this environment does not run. The table is `autoallow/rules.xml`, embedded and checked by `rules.xsd`. MCP trust is per SERVER, not per tool-name pattern.

A rule with `wholeCommand="true"` refuses only a command that runs nothing else. The `true`, `false` and `:` rules work this way. A list that holds real work is never refused for a no-op inside it, because clean-bash repairs that list. A `while` loop on a bare `true` or `:` waits like `until`, and the `until` rule refuses it.

The binary is registered on PermissionRequest and PreToolUse. In `cli.js` (checked at `2.1.220`), `createCanUseTool` evaluates the rules first. PermissionRequest hooks run only on the "ask" path. `defaultMode: "auto"` answers first. A deny that rides PermissionRequest thus never fires in auto mode.

PreToolUse fires before the permission pipeline, whatever its outcome. A deny there stops the tool. The reason reaches the model as an error result. Deny therefore rides PreToolUse. Allow rides PermissionRequest, where it never outranks the user. `denyOnly` in `run.go` drops every non-deny verdict on PreToolUse.

The events want different output objects. `decisionPayload` therefore branches on the event. The verdict comes from `tool_input` alone. A call judged on both events is therefore harmless.

`rules.xml` has no `<mcpServer name="Claude_Code_Remote">` block, on purpose. Do not add one. The CCR proxy answers a call with `-32003` ("needs_approval"). The client then shows an approval card and runs the hooks beside it. A hook allow calls `cancelRequest`, which kills the card and the listener that a human click resolves through. The single retry carries no grant.

The model then sees `MCP error -32003: MCP tool call requires approval`.

Nothing else automates the click. The retry hands `canUseTool` a prebuilt `ask` decision. No rule, mode or classifier runs. `extraAllowedTools` does not exist in the bundle. `serverApprovalWatch` is the shape of a real upstream fix. `anthropics/claude-code#81362` tracks it.

Without `send_later`, use the `/loop` skill. Watch a pull request with `mcp__github__subscribe_pr_activity`.

## The table as a library

`autoallow.Allowed(argv)` judges one simple command against the shipped table, and `autoallow.AllowedMCP(server, tool)` judges one MCP tool. A program with a permission chain of its own calls them, so it allows exactly what these sessions allow. `simple-llm-harness` does. A deny or ask rule that matches makes `Allowed` false, and so does a process rule that names the program. A `wholeCommand` rule judges a whole command line, so `Allowed` leaves it out.

## Rule attributes the engine reads

- A denied or required flag of a single letter also matches inside a cluster: `sort -uo out` carries `-o`, and `tar -tzf` carries `-t`.
- A required flag admits a command to the other checks. It does not skip them: `tar -t -x` still extracts.
- A rule with subcommands skips the leading flags it allows. The next word must then be one of its subcommands. `git -C dir log` reaches `log`. `git remote -v add x` does not fall through to the `-v` that `remote` allows alone.
- `maxPositionals` caps the operands. The second operand of `uniq` is a file it writes.
