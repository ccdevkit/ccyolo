# Model Selection in ccyolo

## Problem

When running Claude Code on the host machine, Max subscribers get Opus 4.5 as the default model. However, when running via ccyolo in a Docker container, it defaults to Sonnet.

## Root Cause

Claude Code determines the default model based on the user's subscription type (`max`, `pro`, `team`, etc.). The relevant code path in `cli.js`:

```javascript
function Aq() {  // Returns subscription type
  if (!rV()) return null;
  let A = iK();  // Get auth object
  if (!A) return null;
  return A.subscriptionType ?? null;
}

function Y1A() {  // Determine default model
  if ($vA() || _vA() || NHA()) return fHA();  // max/team/pro -> Opus
  return Dv();  // fallback -> Sonnet
}
```

The `iK()` function that retrieves auth credentials has this logic:

```javascript
iK = q6(() => {
  if (process.env.CLAUDE_CODE_OAUTH_TOKEN)
    return {
      accessToken: process.env.CLAUDE_CODE_OAUTH_TOKEN,
      refreshToken: null,
      expiresAt: null,
      scopes: ["user:inference"],
      subscriptionType: null,  // <-- Always null when using env var
      rateLimitTier: null,
    };
  // ... otherwise read from keychain/file
});
```

**When `CLAUDE_CODE_OAUTH_TOKEN` is set, the subscription type is hardcoded to `null`**, which causes Claude Code to fall back to Sonnet regardless of actual subscription.

## Why We Can't Fix This

On macOS, the full OAuth credentials (including `subscriptionType`) are stored in the **macOS Keychain**, not in `~/.claude.json`:

```bash
$ security find-generic-password -s "Claude Code-credentials" -w
{
  "claudeAiOauth": {
    "accessToken": "sk-ant-oat01-...",
    "refreshToken": "sk-ant-ort01-...",
    "subscriptionType": "max",
    "rateLimitTier": "default_claude_max_20x"
  }
}
```

The Docker container cannot access the macOS Keychain to retrieve this information.

### Potential Workarounds (Not Implemented)

1. **Read from Keychain on host, write to container's `.claude.json`**
   - Would require running `security` command and parsing keychain data
   - Would expose refresh tokens in a file instead of secure storage
   - Security concern: credentials would be readable in container filesystem

2. **Pass subscription type via environment variable**
   - Claude Code doesn't support this - the env var path hardcodes `subscriptionType: null`
   - Would require patching Claude Code

3. **Use `--model opus` flag**
   - Works, but doesn't "match" the host's behavior
   - User would need to know they're a Max subscriber

## Current Behavior

ccyolo passes the OAuth token via `CLAUDE_CODE_OAUTH_TOKEN`, which works for authentication but loses subscription tier information. The container defaults to Sonnet.

Users who want Opus can use:
```bash
ccyolo --model opus [other args]
```
