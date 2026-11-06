### 2. Keyless fallback (branch 3) ambiguity

- branch (1) the still-streaming `current_agent_msg`, guarded by `streaming_matches`.
- branch (2) the prompt→entry map `finished_prompt_costs` recorded by `finish_turn`.
- branch (3) the keyless fallback `last_finished_agent_entry`.

- during a newer stream → dropped (branch 1 prompt-mismatch rejects, branch 3 sees `last_finished_agent_entry` already cleared).
