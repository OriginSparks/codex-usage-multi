# codex-usage-multi Design

## Goal

Build a tiny compiled CLI for checking Codex usage across multiple OpenAI / ChatGPT accounts.

## Final product shape

Only two user-facing commands matter:

```bash
codex-usage add <email>
codex-usage list
```

## Profile model

Each email is treated as a profile name and gets its own isolated Codex home:

```text
~/.codex-multi/profiles/<email>/.codex
```

## `add <email>`

Behavior:
- validate email format
- create the profile if it does not exist
- if auth is missing, run `codex login` with isolated `CODEX_HOME`
- return after login completes

## `list`

For every email profile:
- read `auth.json`
- extract bearer token defensively
- call `https://chatgpt.com/backend-api/wham/usage`
- map windows by duration
- show remaining percentages and reset times

Example output:

```text
PROFILE                  5H LEFT   5H RESET   1W LEFT   1W RESET
user@example.com         50%       13:30      87%       Mar 17
```

## Window mapping

Confirmed against local CodexBar source:
- 300 minutes => 5h
- 10080 minutes => 1w
- remaining percent = `100 - usedPercent`

## Non-goals

- no TUI
- no profile subcommands for now
- no extra dashboard scraping
- no alerts yet
