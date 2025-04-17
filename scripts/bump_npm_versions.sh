#!/bin/bash

# This script is used to bump the versions of the NIL packages.
# Usage: ./bump_npm_versions.sh <niljs-version> <smart-contracts-version>
# If no arguments are passed, it will grab the latest patch version for each package
# and update all references in all package.json files.

COLOR_END="\033[0m"
COLOR_GREEN="\033[0;32m"
COLOR_YELLOW="\033[0;33m"

pkgs=(
    "niljs"
    "smart-contracts"
)

versions=()

update-versions() {
    echo -e "${COLOR_YELLOW}Updating to versions: ${pkgs[0]}@${versions[0]}, ${pkgs[1]}@${versions[1]}${COLOR_END}"

    files=$(grep -rlE --include=".*/package.json" '@nilfoundation/(niljs|smart-contracts).*":\s*"[\^0-9.]+"' . | grep -v node_modules)

    for f in $files; do
        for ((i = 0; i < 2; i++)); do
            ver="${versions[$i]}"
            pkg="${pkgs[$i]}"

            sed -i '' "s|\(@nilfoundation/$pkg.*\"\)[\^0-9.]\{1,\}\"|\1$ver\"|g" "$f"
        done
    done

    echo -e "${COLOR_GREEN}Versions bumped${COLOR_END}"
}

if [[ "$#" -eq 0 ]]; then
    echo -e "${COLOR_YELLOW}Getting latest versions of packages...${COLOR_END}"
    for pkg in ${pkgs[@]}; do
        version=$(grep '"version"' $pkg/package.json | sed -E 's/.*"version": "(.*)".*/\1/')
        echo "Latest version for $pkg is $version"
        versions+=($version)
    done

    update-versions
elif [[ "$#" -eq 2 ]]; then
    versions=($1 $2)
    update-versions
else
    echo "Usage: $0 <${pkgs[0]}-version> <${pkgs[1]}-version> \
to update to these versions or no arguments to grab the latest patch version for each package"
    exit 1
fi

REPO_DIR=$(readlink -f $(dirname $0)/../)

pushd $REPO_DIR

pnpm install

echo -e "${COLOR_GREEN}Lock-file generated${COLOR_END}"

sed -i'' "s|hash = \".*\";|hash = \"\";|g" nix/npmdeps.nix
hash=$(nix build .#niljs -L 2>&1 | grep -oE 'got:.*sha256-.*' | grep -oE 'sha256-.*')
sed -i'' "s|hash = \"\";|hash = \"$hash\";|g" nix/npmdeps.nix

echo -e "${COLOR_GREEN}Nix hash updated${COLOR_END}"

popd
