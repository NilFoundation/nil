#!/bin/bash

COLOR_END="\033[0m"
COLOR_RED="\033[0;31m"
COLOR_GREEN="\033[0;32m"

pkgs=("niljs" "smart-contracts" "hardhat-plugin")

if [ "$#" -eq 0 ]; then
    echo "Looking for pkg versions..."
    for pkg in ${pkgs[@]}; do
        echo "For package \`$pkg\` found following versions:"
        grep -rohE '@nilfoundation/'${pkg}'":\s*"[\^0-9.]+"' --include='.*/package.json' . |
            grep -oE '[0-9.]+' |
            sort | uniq
    done

    echo -e "${COLOR_GREEN}Now use: $0 <${pkgs[0]}-version> <${pkgs[1]}-version> <${pkgs[2]}-version> to update to these versions${COLOR_END}"
    exit 0
fi

if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <${pkgs[0]}-version> <${pkgs[1]}-version> <${pkgs[2]}-version> to update to these versions"
    exit 1
fi

versions=($1 $2 $3)
files=$(grep -rlE --include=".*/package.json" '@nilfoundation/(niljs|smart-contracts|hardhat-plugin).*":\s*"[\^0-9.]+"' . | grep -v node_modules)

for f in $files; do
    for ((i = 0; i < 3; i++)); do
        ver="${versions[$i]}"
        pkg="${pkgs[$i]}"

        sed -i '' "s|\(@nilfoundation/$pkg.*\"\)[\^0-9.]\{1,\}\"|\1^$ver\"|g" "$f"
    done
done

echo -e "${COLOR_GREEN}Versions bumped${COLOR_END}"

REPO_DIR=$(readlink -f $(dirname $0)/../)

pushd $REPO_DIR

npm run install:clean

echo -e "${COLOR_GREEN}Lock-file generated${COLOR_END}"

sed -i '' "s|hash = \".*\";|hash = \"\";|g" npmdeps.nix
hash=$(nix build .#niljs -L 2>&1 | grep -oE 'got:.*sha256-.*' | grep -oE 'sha256-.*')
sed -i '' "s|hash = \"\";|hash = \"$hash\";|g" npmdeps.nix

echo -e "${COLOR_GREEN}Nix hash updated${COLOR_END}"

popd
