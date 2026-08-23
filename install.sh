#!/usr/bin/env bash
# Fetches docker-compose.yml and generates a .env with random secrets, for
# deploying tally-mcp without cloning the repo:
#
#   curl -fsSL https://raw.githubusercontent.com/chaoliu719/tally-mcp/main/install.sh | bash
set -euo pipefail

REPO="chaoliu719/tally-mcp"
BRANCH="main"
RAW_BASE="https://raw.githubusercontent.com/${REPO}/${BRANCH}"
DEST_DIR="${1:-tally-mcp}"

command -v curl >/dev/null || { echo "error: curl is required" >&2; exit 1; }
command -v openssl >/dev/null || { echo "error: openssl is required" >&2; exit 1; }

mkdir -p "$DEST_DIR"
cd "$DEST_DIR"

echo "Fetching docker-compose.yml..."
curl -fsSL -o docker-compose.yml "${RAW_BASE}/docker-compose.yml"

if [ -f .env ]; then
  echo ".env already exists, leaving it untouched."
else
  echo "Generating .env with random secrets..."
  cat > .env <<EOF
TALLY_MCP_TOKEN=$(openssl rand -hex 32)
TALLY_CONFIRMATION_SECRET=$(openssl rand -hex 32)
TALLY_HOST_PORT=16355
EOF
  chmod 600 .env
fi

cat <<EOF

Done. Next steps:
  cd $(pwd)
  docker compose up -d

TALLY_MCP_TOKEN is your MCP client's bearer token — read it with:
  grep TALLY_MCP_TOKEN .env
EOF
