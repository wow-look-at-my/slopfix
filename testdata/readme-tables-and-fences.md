# Grok


#### How It Works

```
┌──────────────┐     sh -c     ┌────────────────────────┐
│     Grok     │──────────────▶│  your auth binary      │
│              │               │                        │
│  reads       │◀── stdout ────│  prints token          │
│  auth.json   │               │                        │
│              │   (stderr)    │  prints status/URLs    │──▶ user's terminal
└──────────────┘               └────────────────────────┘

#### Minimal Example

```bash
#!/bin/sh
# Print login URL / status to stderr (user sees this)
echo "Authenticating via Acme Corp SSO..." >&2
echo "Visit: https://sso.acme.com/device-login?code=ABCD-1234" >&2

# ... do the auth flow, get a token ...

# Print ONLY the token to stdout (Grok captures this)

Common log messages:

| Log message | What it means |
|-------------|---------------|
| `auth: running external auth provider (headless refresh)` | Your binary is being called with `GROK_AUTH_EXPIRED=1` and a few seconds to work |
| `auth: running external auth provider (interactive login)` | Your binary is being called on the sign-in contract: no `GROK_AUTH_EXPIRED`, stderr shown, 300s |
| `auth: external auth provider returned fresh token` | Success — token was parsed and stored |
| `auth: external auth provider failed` | Binary exited non-zero, or exited 0 but stdout was empty/unparseable (the `error` field has details) |
| `auth: external auth provider timed out (likely needs interactive auth), killing` | Binary didn't exit before the 7s headless-refresh timeout and was killed. Exiting non-zero on `GROK_AUTH_EXPIRED=1` avoids this wait entirely |
| `auth: failed to start external auth provider` | The command couldn't be spawned (e.g. binary not found) |

### Launch

```bash
grok [OPTIONS]
```

### Options

| Flag                       | Description                                                            |
| -------------------------- | ---------------------------------------------------------------------- |
| `--cwd <PATH>`             | Set working directory (default: current directory)                     |
| `--prompt <TEXT>`          | Send an initial prompt immediately after startup                       |
| `--rules <TEXT>`           | Append custom rules to the system prompt                               |
| `--always-approve`         | Auto-approve all tool executions without confirmation                  |
| `--sandbox <PROFILE>`      | OS-level filesystem/network guardrails (see [Sandbox](#sandbox))       |
| `--light`                  | Use light theme (macOS Basic) instead of dark                          |
| `--single-turn`            | Exit after first response (requires `--prompt`)                        |
| `--no-memory`              | Force-disable cross-session memory (overrides all other settings)      |
| `--subagents`              | Enable subagent/task tool support (see [Subagents](#subagents))        |
| `--disable-web-search`     | Remove web search tool from the agent toolset                          |
| `--agent-profile <PATH>`   | Load a custom agent definition file (see [Agent Profiles](#agent-profiles)) |
| `--experimental-memory`    | Enable cross-session memory persistence (see [Memory](#memory))        |
| `--allow <RULE>`           | Permission allow rule with glob patterns (repeatable). See [Permission Rules](#permission-rules-allow--deny). |
| `--deny <RULE>`            | Permission deny rule with glob patterns (repeatable). See [Permission Rules](#permission-rules-allow--deny). |

### Examples

```bash
# Start in a specific project
grok --cwd ~/projects/my-app

# Start with an initial task
grok --prompt "Review this codebase and suggest improvements"

# Add project-specific rules
grok --rules "Always use TypeScript. Prefer functional components."

# Auto-approve mode for trusted tasks
grok --always-approve --prompt "Format all files"
```

### Keyboard Shortcuts

| Key                          | Action                          |
| ---------------------------- | ------------------------------- |
| `Enter`                      | Send message                    |
| `Shift+Enter` or `Alt+Enter` | Insert newline                  |
| `Ctrl+M`                     | Toggle multiline input mode     |
| `Ctrl+C` or `Esc`            | Cancel current operation        |
| `Ctrl+D` or `Ctrl+Q`         | Quit (with confirmation)        |
| `Ctrl+O`                     | Toggle always-approve mode |
| `Ctrl+T`                     | Toggle TODO/task panel          |
| `Ctrl+R`                     | Search prompt history           |
| `Ctrl+V`                     | Paste from clipboard            |
| `Ctrl+U`                     | Undo last input change          |
| `Ctrl+G`                     | Move foreground task to background |
| `Ctrl+P`                     | Toggle debug panel              |

### Slash Commands
