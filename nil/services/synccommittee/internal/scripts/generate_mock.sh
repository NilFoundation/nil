#!/bin/bash

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <GoInterfaceName>"
    exit 1
fi

INTERFACE_NAME="$1"

FILE_CONTENT="//go:build test
$(go run github.com/matryer/moq -rm -stub -with-resets . "$INTERFACE_NAME")"

snake_case() {
    echo "$1" | sed -E 's/([a-z])([A-Z])/\1_\2/g' | tr '[:upper:]' '[:lower:]'
}

echo "$FILE_CONTENT" >"$(snake_case "$INTERFACE_NAME")_generated_mock.go"
