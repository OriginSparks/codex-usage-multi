# codex-usage-multi

A small Go CLI for checking Codex usage across multiple isolated accounts.

## Final UX

The tool is intentionally simple:

- `codex-usage add <email>`
- `codex-usage list`

Each email becomes its own isolated profile under:

```text
~/.codex-multi/profiles/<email>/.codex
```

## Commands

### Add an account

```bash
codex-usage add user@example.com
```

Behavior:
- uses the email as the profile name
- creates the profile if missing
- if the profile is not logged in yet, runs `codex login` with an isolated `CODEX_HOME`
- returns after login completes

This implementation uses `codex login`, so it does **not** intentionally drop the user into the normal Codex interactive shell after login.

### List current usage

```bash
codex-usage list
```

Example output:

```text
PROFILE                  5H LEFT   5H RESET   1W LEFT   1W RESET
user@example.com         50%       13:30      87%       Mar 17
```

Meaning:
- `5H LEFT`: remaining percentage for the official 5-hour window
- `1W LEFT`: remaining percentage for the official 1-week window
- reset times are shown as `HH:MM` for same-day resets and `Mon DD` for later resets

## How usage is mapped

The implementation follows the same window-duration mapping used by CodexBar.

Usage is fetched from:

```text
https://chatgpt.com/backend-api/wham/usage
```

Windows are identified by `limit_window_seconds`:
- `300 minutes` => `5h`
- `10080 minutes` => `1w`

Remaining percentage is calculated as:

```text
100 - usedPercent
```

## Build

### GitHub Actions artifacts

The repository includes a GitHub Actions workflow that:
1. runs `go test ./...`
2. builds binaries for macOS arm64, macOS amd64, and Linux amd64
3. uploads `.tar.gz` artifacts

That means the runtime machine does **not** need Go installed if you use the generated artifacts.

### Local build

If you do want to build locally:

```bash
go build -o bin/codex-usage ./cmd/codex-usage
```

## Requirements at runtime

You do **not** need Go installed to run the compiled binary.

You **do** need the `codex` CLI installed if you want to use:

```bash
codex-usage add <email>
```

because login is delegated to `codex login`.

## Notes

- This is not an official OpenAI tool.
- The local `auth.json` format may change over time.
- The `wham/usage` endpoint may change over time.
- The tool does not print tokens.
- Profiles using API-key auth instead of ChatGPT login may not work with this endpoint.
