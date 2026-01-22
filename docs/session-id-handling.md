# Session ID Handling in ccyolo

## Overview

ccyolo needs to manage two different concepts:
1. **Claude's session tracking**: Claude Code uses session IDs to persist conversation history in `~/.claude/projects/{project-path}/{session-id}.jsonl`
2. **ccyolo's temp files**: ccyolo creates temporary files (settings, proxy config, system prompt) that need a unique directory

## The Problem

Originally, ccyolo generated a session ID and passed it to Claude via `--session-id`. This caused issues with Claude's `-c`/`--continue` and `-r`/`--resume` flags:

```
$ ccyolo -c
Error: --session-id can only be used with --continue or --resume if --fork-session is also specified.
```

The issue is that Claude expects:
- `-c`/`--continue`: Resume the **most recent** session (no explicit session ID needed)
- `-r`/`--resume <id>`: Resume a **specific** session
- `-r`/`--resume` (no ID): Show an interactive session picker

When we pass `--session-id`, it conflicts with these modes.

## Current Solution

**Never pass `--session-id` to Claude.** Let Claude manage its own sessions entirely.

ccyolo generates its own UUID purely for organizing temporary files (settings.json, proxy config, etc.) in a temp directory. This UUID has no relationship to Claude's session ID.

### Implementation

1. ccyolo generates a UUID for its temp directory: `/tmp/{uuid}/`
2. ccyolo **never** passes `--session-id` to Claude
3. Claude manages its own session IDs based on user flags (`-c`, `-r`, etc.)

### Trade-offs

**Pros:**
- Simplest implementation
- Works with all Claude session modes without special handling
- No need to understand or replicate Claude's session logic

**Cons:**
- ccyolo's temp dir UUID doesn't match Claude's session ID
- Slightly harder to correlate debug logs with Claude sessions

## Future Considerations

If we ever need to know Claude's session ID (e.g., for persistent per-session state), options include:

1. **Two-phase startup**: Start Claude, detect which session was chosen, then set up files
   - Would need to parse TUI output or watch for file changes

2. **Watch sessions-index.json**: After Claude starts, monitor the index for new/updated entries
   - Race conditions, complexity

3. **Claude Code API**: If Claude exposes session info via API or environment variable
   - Would be the cleanest solution if available

## Reference: Claude's Session Storage

Claude stores session metadata in `~/.claude/projects/{project-path}/sessions-index.json`:

```json
{
  "version": 1,
  "entries": [
    {
      "sessionId": "d8678f90-4d42-4c2f-89ee-cb165576b100",
      "fullPath": "/Users/.../{session-id}.jsonl",
      "fileMtime": 1769067832872,
      "firstPrompt": "...",
      "summary": "...",
      "messageCount": 15,
      "created": "2026-01-22T07:35:51.866Z",
      "modified": "2026-01-22T07:43:52.788Z",
      "gitBranch": "main",
      "projectPath": "/Users/brad/Development/...",
      "isSidechain": false
    }
  ]
}
```

The most recent session is determined by the highest `fileMtime` value.

**Project path encoding:** Claude encodes the project path by replacing `/` with `-` and `.` with `-`.
Example: `/Users/brad/Development/foo` becomes `-Users-brad-Development-foo`
