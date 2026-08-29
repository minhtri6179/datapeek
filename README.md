# datapeek

[![CI](https://github.com/minhtri6179/datapeek/actions/workflows/ci.yml/badge.svg)](https://github.com/minhtri6179/datapeek/actions/workflows/ci.yml)

A lightweight desktop database viewer with a fast virtualized datagrid. Built with Go (Wails v2) + React.

## Features (v0.1)

- **MySQL & PostgreSQL** — browse schemas, tables, and views
- **Virtualized datagrid** — server-side pagination (up to 500 rows/page) and column sorting
- **Cell detail viewer** — full values with JSON pretty-printing and binary hex
- **Secure secrets** — passwords stored in the OS keychain, never on disk
- **Structured logging** — JSON logs with correlation IDs under `~/.datapeek/logs/` (SQL values are never logged)

## Stack

| Layer | Choice |
|---|---|
| Shell | Wails v2 |
| Backend | Go 1.22+ (`database/sql`) |
| Drivers | `go-sql-driver/mysql`, `jackc/pgx/v5` (stdlib mode) |
| Frontend | React 19 + TypeScript + Vite |
| Grid | TanStack Virtual |
| State | Zustand |
| Secrets | `zalando/go-keyring` |

## Development

```bash
# Prerequisites: Go 1.22+, Node 18+, Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

wails dev     # hot-reload dev mode
wails build   # production build → build/bin/
go test ./... # backend tests
```

## Roadmap

- v0.5: SQL query console (CodeMirror, history, read-only guard)
- v1.0: Redis key browser + typed viewers, observability Diagnostics panel (metrics, traces), packaging/CI

## Configuration

Connections live in `~/.datapeek/connections.json` (passwords in the OS keychain). Logs rotate at 10MB × 5 files, pruned after 7 days.

## Telegram notifications

CI and Release workflows send a Telegram message when builds succeed. Add two repository secrets (**Settings → Secrets and variables → Actions**):

| Secret | Value |
|---|---|
| `TELEGRAM_BOT_TOKEN` | Bot token from [@BotFather](https://t.me/BotFather) |
| `TELEGRAM_CHAT_ID` | Your chat id — send the bot a message, then check `https://api.telegram.org/bot<TOKEN>/getUpdates` for `"chat":{"id":...}` |

Test locally before pushing:

```bash
export TELEGRAM_BOT_TOKEN="123456:ABC..."
export TELEGRAM_CHAT_ID="123456789"
./scripts/notify-telegram.sh "hello from datapeek"
```
