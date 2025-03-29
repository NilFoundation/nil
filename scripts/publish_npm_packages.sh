#!/bin/bash

# This script publishes packages to npm registry making them publicly available.
# First it checks if the packages are already published, then it runs the publish command.
# It publishes only packages that have 'pub' script in their package.json file.
# Script accepts pnpm workspaces names as arguments. E.g. `./scripts/publish_npm_packages.sh my-workspace`

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

if ! command -v pnpm >/dev/null 2>&1; then
    echo "pnpm should be installed to run this script."
    exit 1
fi

if ! command -v jq &>/dev/null; then
    echo "jq is not installed, please install it before running this script."
    exit 1
fi

cd "$REPO_ROOT"

for package in "$@"; do
    cd "$REPO_ROOT/$package"

    PACKAGE_JSON="./package.json"

    if [[ ! -f "$PACKAGE_JSON" ]]; then
        echo "⚠️ No package.json found in $package, skipping..."
        continue
    fi

    PACKAGE_NAME=$(jq -r '.name' "$PACKAGE_JSON")
    LOCAL_VERSION=$(jq -r '.version' "$PACKAGE_JSON")

    PUBLISHED_VERSION=$(pnpm show "$PACKAGE_NAME" version 2>/dev/null || echo "not_published")

    if [[ "$PUBLISHED_VERSION" == "not_published" ]]; then
        echo -e "🚀 Package ${COLOR_YELLOW}$PACKAGE_NAME${COLOR_END} is not published yet!"
    elif [[ "$LOCAL_VERSION" == "$PUBLISHED_VERSION" ]]; then
        echo -e "✅ Package ${COLOR_YELLOW}$PACKAGE_NAME${COLOR_END} is already published (version $LOCAL_VERSION)."
    else
        echo -e "🔄 Package ${COLOR_YELLOW}$PACKAGE_NAME${COLOR_END} has a new version ($LOCAL_VERSION) compared to npm ($PUBLISHED_VERSION)."

        echo -e "${COLOR_YELLOW}Publishing $PACKAGE_NAME...${COLOR_END}"
        pnpm run pub
        echo -e "${COLOR_GREEN}Published ${COLOR_YELLOW}$PACKAGE_NAME${COLOR_END}"
    fi
done
