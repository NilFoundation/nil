#!/bin/bash

# This script publishes packages to npm registry making them publicly available.
# First it checks if the packages are already published, then it runs the publish command.
# It publishes only packages that have 'pub' script in their package.json file.

set -euo pipefail

COLOR_END="\033[0m"
COLOR_GREEN="\033[0;32m"
COLOR_YELLOW="\033[0;33m"

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
REPO_ROOT="$SCRIPT_DIR/.."

if ! command -v node >/dev/null 2>&1; then
    echo "nodejs should be installed to run this script."
    exit 1
fi

if ! command -v npm >/dev/null 2>&1; then
    echo "npm should be installed to run this script."
    exit 1
fi

if ! command -v jq &>/dev/null; then
    echo "jq is not installed, please install it before running this script."
    exit 1
fi

cd "$REPO_ROOT"
NPM_WORKSPACES=$(npm pkg get workspaces | jq -r '.[]')

for workspace in $NPM_WORKSPACES; do
    cd "$REPO_ROOT/$workspace"

    PACKAGE_JSON="./package.json"

    if [[ ! -f "$PACKAGE_JSON" ]]; then
        echo "⚠️ No package.json found in $workspace, skipping..."
        continue
    fi

    if jq -e '.scripts.pub' "$PACKAGE_JSON" >/dev/null; then
        echo "'pub' script found in $workspace/package.json"
    else
        continue
    fi

    PACKAGE_NAME=$(jq -r '.name' "$PACKAGE_JSON")
    LOCAL_VERSION=$(jq -r '.version' "$PACKAGE_JSON")

    PUBLISHED_VERSION=$(npm show "$PACKAGE_NAME" version 2>/dev/null || echo "not_published")

    if [[ "$PUBLISHED_VERSION" == "not_published" ]]; then
        echo -e "🚀 Package ${COLOR_YELLOW}$PACKAGE_NAME${COLOR_END} is not published yet!"
    elif [[ "$LOCAL_VERSION" == "$PUBLISHED_VERSION" ]]; then
        echo -e "✅ Package ${COLOR_YELLOW}$PACKAGE_NAME${COLOR_END} is already published (version $LOCAL_VERSION)."
    else
        echo -e "🔄 Package ${COLOR_YELLOW}$PACKAGE_NAME${COLOR_END} has a new version ($LOCAL_VERSION) compared to npm ($PUBLISHED_VERSION)."

        echo -e "${COLOR_YELLOW}Publishing $PACKAGE_NAME...${COLOR_END}"
        npm run pub
        echo -e "${COLOR_GREEN}Published ${COLOR_YELLOW}$PACKAGE_NAME${COLOR_END}"
    fi
done
