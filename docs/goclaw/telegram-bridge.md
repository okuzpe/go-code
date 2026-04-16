# Telegram bridge (optional)

Run goclaw from Telegram for **your** account only: the bridge refuses to start unless you configure a **non-empty allowlist** of Telegram numeric user ids. Incoming messages from anyone else are ignored (no agent run, no reply).

## What it does

`goclaw telegram start` and **`goclaw telegram bridge`** both long-poll the [Telegram Bot API](https://core.telegram.org/bots/api) (`getUpdates` / `sendMessage` against `https://api.telegram.org` only). For each **private text** message from an allowlisted user, goclaw runs **one** orchestrator turn (same stack as `goclaw prompt`) and sends the final assistant text back, split into chunks of up to 4096 characters.

| Command | When to use |
|--------|--------------|
| **`goclaw telegram start`** | Default for humans: if bot token or allowlist is missing **and** stdin/stdout are TTYs, runs a short interactive wizard that **merges** keys into `~/.goclaw/settings.local.json`, then starts the bridge. |
| **`goclaw telegram configure`** | Only the wizard (merge settings); does not start the bridge. |
| **`goclaw telegram bridge`** | Strict: exits immediately if settings are incomplete (for scripts, CI, or when you already edited JSON / env). |

This path **cannot** show interactive tool-approval prompts. Tools that are in **ask** mode will fail the turn with an error (same behavior as `--output-format json`). For remote use, set `tool_permissions` to **`allow`** for the tools you need, use a **read-only** profile, or **`--no-tools`**.

## Setup

1. Create a bot with [@BotFather](https://t.me/BotFather) and copy the **HTTP API token**.
2. Identify **your personal** Telegram account for the allowlist:
   - **Numeric user id** (e.g. `123456789` or `#123456789` if your client copies it with a hash): from Telegram → **Settings**, or bots like **@userinfobot**, or any client that shows id.
   - **Or `@YourPublicUsername`**: during `goclaw telegram configure` / `telegram start`, goclaw calls Bot API **`getChat`** to resolve `@username` to an id. This only works for **public** usernames; Telegram may reject hidden or invalid handles.
   - **Not** your bot's `@BotName`: that resolves to the **bot's** id, which is rejected (it would never match `message.from.id` when *you* write to the bot). **Do not** run the bridge with an empty allowlist.
3. Put secrets in **`~/.goclaw/settings.local.json`** (not committed), for example:

```json
{
  "telegram_bot_token": "123456:ABC-DEF…",
  "telegram_allowed_user_ids": [987654321],
  "telegram_session_id": "optional-session-uuid-to-reuse"
}
```

Alternatively:

- **`telegram_bot_token_file`**: path to a file containing a single line (token). Relative paths resolve from the process **current working directory** when goclaw starts.
- **`GOCLAW_TELEGRAM_BOT_TOKEN`**: overrides the merged token when set (after JSON load).
- **`GOCLAW_TELEGRAM_ALLOWED_USER_IDS`**: comma-separated list of integers; when set and non-empty, it **replaces** the JSON allowlist after merge.

If **`--session`** is omitted and **`telegram_session_id`** is set in settings, the bridge resumes that session id (same as passing **`--session`**).

## Run

From your project directory (so `.goclaw/` settings and workspace match what you expect):

```bash
goclaw telegram start
```

From the `goclaw` module root, **`make telegram`** runs `telegram start` (guided setup when needed).

Use the same **persistent flags** as other commands: **`--profile`**, **`--session`**, **`--workspace`**, **`--no-tools`**, etc.

Stop with Ctrl+C or SIGTERM.

## Troubleshooting (“no reply” in Telegram)

1. **Watch the terminal** where the bridge runs (default log level is `info`). After you send a message you should see either:
   - `telegram bridge: received updates` then `telegram bridge: running turn` and `telegram bridge: reply sent`, or
   - `telegram bridge: ignored message (sender not in telegram_allowed_user_ids)` with **`from_user_id`**. If that number differs from what you put in `telegram_allowed_user_ids`, update settings to match **`from_user_id`** (e.g. confirm with [@userinfobot](https://t.me/userinfobot)).
2. **Chat with the bot in private**, with **plain text** (stickers, photos only, or group chats without addressing the bot often produce no handled message).
3. **`GOCLAW_LOG=debug`** for more detail (including skipped non-text updates).
4. If you see **`turn error`** in Telegram, the model or tools failed (e.g. Ollama down, or a tool in **ask** mode — see the table above). Fix the error text shown in the chat or in the terminal.

## Security

- The **bot token** is equivalent to a password for the bot: anyone with it can call the Bot API as your bot. Keep it in `settings.local.json` or a chmod-restricted token file; never commit it.
- The **allowlist** is mandatory: without `telegram_allowed_user_ids` (or env equivalent), the bridge exits immediately.
- Outbound traffic is only to **Telegram’s API host** from the built-in client; there is no user-supplied base URL.

See also [security.md](./security.md) and `goclaw doctor` for a masked summary (token present / allowlist count only).
