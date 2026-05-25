# Discord Bot Blueprint (Python / discord.py)

A minimal `discord.py` v2 bot with both prefix and slash commands,
sliced into cogs.

## Commands

| Command             | Type   | Purpose                                  |
|---------------------|--------|------------------------------------------|
| `!ping` / `/ping`   | both   | Reply with `pong` + websocket latency.   |
| `!echo <text>`      | prefix | Echo the text back to the channel.       |
| `/echo text:<text>` | slash  | Echo the text back via slash command.    |

## Create your bot

1. Open the [Discord Developer Portal](https://discord.com/developers/applications),
   create an application, and add a Bot.
2. Reset the bot token and copy it into `.env` as `DISCORD_BOT_TOKEN`.
3. Under **Privileged Gateway Intents**, enable **Message Content
   Intent** (the `!`-prefixed commands need it).
4. Generate an OAuth2 URL with the `bot` and `applications.commands`
   scopes plus the permissions you need (Send Messages at minimum).

## Run locally

```bash
cp .env.example .env
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
python bot.py
```

## Run in Docker

```bash
docker build -t ironflyer-discord-bot .
docker run --rm --env-file .env ironflyer-discord-bot
```

## Adding commands

Drop a new cog file under `commands/`, expose an `async def setup(bot)`
loader, and load it from `bot.py` with
`await bot.load_extension("commands.<modname>")`.
