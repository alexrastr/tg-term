# tg-term

`tg-term` is a terminal chat UI for a Telegram bot. It lets you read incoming messages, reply from the terminal, receive an alarm popup on `/alarm`, and see runtime/system errors directly in the chat window.

## Features

- terminal UI built with `tview` and `tcell`
- incoming Telegram messages are shown in `chatView`
- outgoing messages are sent from the input field on `Enter`
- chat history is persisted locally in `BadgerDB` and capped at the latest 200 messages
- `/alarm` triggers a modal window and repeating terminal bell
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
```

Variables:

- `BOT_TOKEN` is required
- `OWNER_ID` is required
- `PROXY_URL` is optional; if empty, the bot uses a regular HTTP client without proxy

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
- each incoming message plays a terminal bell
- when `/alarm` is received, the app shows a modal and repeats the bell until you confirm with `Enter`

## Error Handling

Runtime errors are sent to the chat as `System` messages. This includes:

- invalid or missing `BOT_TOKEN`
- invalid or missing `OWNER_ID`
- invalid `PROXY_URL`
- Telegram send failures
- Telegram polling failures reported by the library
- conflict when the same bot token is used by another running process

If two processes start polling the same bot token at the same time, Telegram stops one of them. In this project that situation is shown in the chat as a system error instead of only being printed to the console.

## Project Structure

```text
.
|-- main.go
|-- bot/
|   `-- telegram.go
|-- storage/
|   `-- messages.go
|-- .env
|-- env.example
`-- README.md
```

## Notes

- only the configured `OWNER_ID` can talk to the bot through the terminal UI
- unauthorized users receive a Telegram reply explaining that the bot is owner-only
- messages are stored locally in `data/messages`, only the latest 200 are kept
- after the bot stops, the app attempts to restart polling after a short delay
