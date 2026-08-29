#!/usr/bin/env bash
# Send a Telegram message via bot API.
# Usage: notify-telegram.sh "<message>"  (HTML parse mode)
# Required env:
#   TELEGRAM_BOT_TOKEN - bot token from @BotFather
#   TELEGRAM_CHAT_ID   - chat to send to (user/group/channel id)
set -euo pipefail

TOKEN="${TELEGRAM_BOT_TOKEN:?TELEGRAM_BOT_TOKEN is not set}"
CHAT_ID="${TELEGRAM_CHAT_ID:?TELEGRAM_CHAT_ID is not set}"
TEXT="${1:?usage: notify-telegram.sh <message>}"

resp=$(curl -sS --fail-with-body -X POST \
  "https://api.telegram.org/bot${TOKEN}/sendMessage" \
  --data-urlencode "chat_id=${CHAT_ID}" \
  --data-urlencode "parse_mode=HTML" \
  --data-urlencode "disable_web_page_preview=true" \
  --data-urlencode "text=${TEXT}") || {
    echo "Telegram notification failed (curl error)" >&2
    exit 1
  }

if ! printf '%s' "$resp" | grep -q '"ok":true'; then
  echo "Telegram API rejected the message: $resp" >&2
  exit 1
fi

echo "Telegram notification sent."
