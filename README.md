# tg-term

`tg-term` is a terminal chat UI for a Telegram bot. It lets you read incoming messages, reply from the terminal, receive an alarm popup on `/alarm`, and see runtime/system errors directly in the chat window.

## Features

- terminal UI built with `tview` and `tcell`
- incoming Telegram messages are shown in `chatView`
- outgoing messages are sent from the input field on `Enter`
- chat history is persisted locally in `BadgerDB` and capped at the latest 200 messages
- `/alarm` triggers a modal window and repeating alert sound
- `/clear` clear chat
- custom commands from `scripts.d/*.sh` and `scripts.d/*.ps1` are available as `/name args`
- system and bot errors are rendered in the chat instead of only going to the console
- access is limited to the configured bot owner

## Requirements

- Go `1.25.4`
- Telegram bot token from BotFather
- Telegram numeric user ID for the bot owner
- optional SOCKS5 proxy

## Configuration

Create a `.env` file in the project root:

```env
BOT_TOKEN=1234567890:YOUR_BOT_TOKEN
PROXY_URL=socks5://127.0.0.1:1080
OWNER_ID=1234567890
APP_LANG=ru
ALERT_SOUND=terminal
```

Variables:

- `BOT_TOKEN` is required
- `OWNER_ID` is required
- `PROXY_URL` is optional; if empty, the bot uses a regular HTTP client without proxy
- `APP_LANG` is optional; can be "ru"
- `ALERT_SOUND` is optional; supported values: `terminal`, `pc-speaker`, `off`
  - on Linux, `pc-speaker` tries the `beep` utility first, then a direct console speaker ioctl; if neither is available, it falls back to the terminal bell

You can copy the template from `env.example`.

## Run

```bash
go run .
```

## How It Works

- the upper panel is the chat history
- previous messages are restored from the local database on startup
- the bottom field is the input line
- press `Enter` to send a message
- press `Esc` to exit the app
- each incoming message plays the configured alert sound
- when `/alarm` is received, the app shows a modal and repeats the configured sound until you confirm with `Enter`

## Error Handling

Runtime errors are sent to the chat as `System` messages. This includes:

- invalid or missing `BOT_TOKEN`
- invalid or missing `OWNER_ID`
- invalid `PROXY_URL`
- Telegram send failures
- Telegram polling failures reported by the library
- conflict when the same bot token is used by another running process


## Notes

- only the configured `OWNER_ID` can talk to the bot through the terminal UI
- unauthorized users receive a Telegram reply explaining that the bot is owner-only
- messages are stored locally in `data/messages`, only the latest 200 are kept
- after the bot stops, the app attempts to restart polling after a short delay

## Custom Commands

If a script exists in `scripts.d`, it becomes available as a slash-command with the script name.

Supported formats:

- `scripts.d/<name>.sh`
- `scripts.d/<name>.ps1`

Example bash script:

```bash
scripts.d/hello.sh
```

```bash
#!/bin/bash
echo "hello!"
```

Then:

```text
/hello
```

Arguments are passed through as-is:

```text
/light on
```

This runs:

```bash
scripts.d/light.sh on
```

PowerShell works the same way:

```powershell
scripts.d/light.ps1
```

```powershell
param(
    [string]$State
)

Write-Output "light: $State"
```

Behavior:

- local command execution prints the result into the terminal chat
- Telegram command execution prints the result into the terminal chat and sends the result back to Telegram
- if a script exits with an error, combined stdout/stderr is shown in chat
- on Windows, if both `.ps1` and `.sh` exist for the same command, `.ps1` is preferred

- `.sh` requires `bash` in `PATH`
- `.ps1` uses `powershell -NoProfile -ExecutionPolicy Bypass -File`
