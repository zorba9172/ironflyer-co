"""Admin cog: !echo prefix command + /echo slash command.

The slash command is registered on the bot's app-command tree in
``setup``; the prefix command is provided by the Cog itself.
"""
from __future__ import annotations

import discord
from discord import app_commands
from discord.ext import commands


class Admin(commands.Cog):
    def __init__(self, bot: commands.Bot) -> None:
        self.bot = bot

    @commands.command(name="echo", help="Repeat the given text back to the channel.")
    async def echo_prefix(self, ctx: commands.Context, *, text: str = "") -> None:
        if not text.strip():
            await ctx.reply("usage: !echo <message>")
            return
        await ctx.send(text)


async def setup(bot: commands.Bot) -> None:
    await bot.add_cog(Admin(bot))

    @bot.tree.command(name="echo", description="Repeat the given text back to the channel.")
    @app_commands.describe(text="The text to echo back.")
    async def echo_slash(interaction: discord.Interaction, text: str) -> None:
        await interaction.response.send_message(text)
