# !/usr/bin/env bash

set -ueo pipefail

ARGS=$(getopt -o 'nc' -n 'sh_format_all.sh' -- "$@")

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
    hash shfmt 2>/dev/null || {
        echo >&2 "shfmt is not installed. Try running me from nix develop shell!"
        exit 1
    }
fi

find . -type f -name '*.sh' ! -path '*/vendor/*' |
    xargs -r $NIXRUN shfmt --indent=4 --language-dialect=bash --diff .

$CHECK_CMD
