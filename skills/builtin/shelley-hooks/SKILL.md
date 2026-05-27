---
name: shelley-hooks
description: Use when the user wants to customize Shelley by injecting behavior at lifecycle events. It documents Shelley's hooks.
---

Executable files at `~/.config/shelley/hooks/<name>`. Missing or non-executable files are ignored. 30s timeout. Any hook failure (non-zero exit, invalid output, etc.) aborts the operation it belongs to — except `end-of-turn`, where the operation is already finished, so failures are just logged.

Auth-bearing headers (`Cookie`, `Set-Cookie`, `Authorization`, `Proxy-Authorization`) are stripped from the `headers` fields before hooks see them.

## `system-prompt`

Runs on every system prompt (main, subagent, orchestrator, orchestrator-subagent).

- stdin: prompt text
- stdout: replacement prompt text (non-empty)

## `new-conversation`

Runs once when a conversation is created: user-initiated or the first run of a new subagent.

stdin JSON:
```json
{
  "prompt": "...", "model": "...", "cwd": "...",
  "readonly": {
    "conversation_id": "cXXXXXX",
    "is_subagent": false, "parent_id": "...",
    "is_orchestrator": false,
    "headers": [["X-Exedev-Email", "user@example.com"]]
  }
}
```
`parent_id` is `omitempty`. `headers` is a sorted list of `[name, value]` pairs (multi-valued headers produce multiple pairs); omitted for subagent and other non-HTTP entry points.

stdout: same top-level shape. Only `prompt`/`model`/`cwd`/`slug` are read; empty fields mean no change; `readonly` is ignored. Empty stdout = no-op.

Applied when non-empty and changed:
- `cwd` → conversation's working directory
- `model` → re-resolves LLM service; falls back to original if unsupported
- `prompt` → first user message (ignored on distillation paths)
- `slug` → sanitized to a slug-safe form; falls back to async slug on collision

## `chat-message`

Fires when the user posts a follow-up chat message to an existing conversation (`POST /api/conversation/<id>/chat`). For the first message of a brand-new conversation, use `new-conversation`. Not fired for subagent conversations.

stdin JSON:
```json
{
  "message": "the user's chat message",
  "readonly": {
    "conversation_id": "cXXXXXX",
    "model": "claude-sonnet-4.5",
    "queued": false,
    "headers": [["X-Exedev-Email", "user@example.com"]]
  }
}
```
`queued` is true when the message will be queued (client requested queue mode or the agent is distilling) rather than interrupting the current turn.

stdout: `{"message": "..."}`. Empty stdout, empty `message`, or an identical `message` means no change.

## `end-of-turn`

Fires when an agent finishes a turn — the same signal that drives end-of-turn
notifications (notification channels, push notifications, conversation-hook
webhooks). Suppressed for subagent conversations. Stdout is ignored.

stdin JSON:
```json
{
  "type": "end_of_turn",
  "conversation_id": "cXXXXXX",
  "timestamp": "2024-01-02T03:04:05Z",
  "hostname": "host.exe.xyz",
  "model": "claude-sonnet-4.5",
  "slug": "my-slug",
  "conversation_url": "https://host.exe.xyz/c/my-slug",
  "vm_name": "host",
  "final_response": "agent's last text or tool-call summary"
}
```

Typical uses: play a sound, post a desktop notification, ping a local script.
