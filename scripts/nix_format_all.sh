# !/usr/bin/env bash

set -ueo pipefail

ARGS=$(getopt -o 'nc' -n 'nix_format_all.sh' -- "$@")

if [ $? -ne 0 ]; then
    exit 1
fi

eval set -- "$ARGS"
unset ARGS

CHECK_CMD=true
NIXRUN=""

while true; do
    case "$1" in
    -n)
        NIXRUN="nix develop .#formatters -c"
        shift
        continue
        ;;
    -c)
        CHECK_CMD="git diff --exit-code"
        shift
        continue
        ;;
    '--')
        shift
        break
        ;;
    *)
        echo "Usage: $0 [-n] [-c]"
        exit 1
        ;;
    esac
done

if [ -z "$NIXRUN" ]; then
    hash nixpkgs-fmt 2>/dev/null || {
        echo >&2 "nixpkgs-fmt is not installed. Try running me from nix develop shell!"
        exit 1
    }
fi

git ls-tree -r --name-only HEAD |
    sed -n 's#\.nix$#&#p' |
    xargs -r $NIXRUN nixpkgs-fmt

$CHECK_CMD
