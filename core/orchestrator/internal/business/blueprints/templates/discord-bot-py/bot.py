"""Ironflyer Discord bot blueprint.

Reads DISCORD_BOT_TOKEN from the environment, registers a !ping prefix
command, a /ping slash command, and loads the `commands.admin` cog
which exposes !echo and /echo.
"""
from __future__ import annotations

import asyncio
import logging
import os
import sys

import discord
from discord import app_commands
from discord.ext import commands

LOG_LEVEL = os.environ.get("LOG_LEVEL", "INFO").upper()
logging.basicConfig(level=LOG_LEVEL, format="%(asctime)s %(levelname)s %(name)s :: %(message)s")
log = logging.getLogger("ironflyer.bot")

COMMAND_PREFIX = os.environ.get("BOT_PREFIX", "!")

intents = discord.Intents.default()
intents.message_content = True
intents.members = False

bot = commands.Bot(command_prefix=COMMAND_PREFIX, intents=intents)


@bot.event
async def on_ready() -> None:
    log.info("logged in as %s (id=%s)", bot.user, getattr(bot.user, "id", "?"))
    try:
        synced = await bot.tree.sync()
        log.info("synced %d slash command(s)", len(synced))
    except Exception:
        log.exception("failed to sync slash commands")


@bot.command(name="ping", help="Reply with pong and the websocket latency.")
async def ping_prefix(ctx: commands.Context) -> None:
    latency_ms = round(bot.latency * 1000)
    await ctx.reply(f"pong — {latency_ms}ms")


@bot.tree.command(name="ping", description="Reply with pong and the websocket latency.")
async def ping_slash(interaction: discord.Interaction) -> None:
    latency_ms = round(bot.latency * 1000)
    await interaction.response.send_message(f"pong — {latency_ms}ms", ephemeral=True)


async def main() -> int:
    token = os.environ.get("DISCORD_BOT_TOKEN")
    if not token:
        log.error("DISCORD_BOT_TOKEN is required; refusing to start")
        return 2

    await bot.load_extension("commands.admin")

    try:
        await bot.start(token)
    except KeyboardInterrupt:
        log.info("shutting down")
        await bot.close()
    return 0


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))
