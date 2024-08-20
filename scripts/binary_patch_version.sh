#!/usr/bin/env bash

set -e

FILE_TO_PATCH="$1"
VERSION="$2"
VERSION_LEN=${#VERSION}

if [ ! -f "$FILE_TO_PATCH" ]; then
    echo "ERROR: File doesn't exist: $FILE_TO_PATCH"
    exit 1
fi

if [ "$VERSION_LEN" -gt 40 ]; then
    echo "ERROR: Version is longer than 40 characters: " $VERSION
    exit 1
fi

PADDED_VERSION="$(printf "%-40s" $VERSION)"
VERSION_MAGIC="qm5h7IEa3ahXUgsPknK8bwWulPEmpgMWSaQSaOUa"

echo "Patching version string in $FILE_TO_PATCH"
sed -i "s/$VERSION_MAGIC/$PADDED_VERSION/g" "$FILE_TO_PATCH"
