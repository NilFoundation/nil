#!/bin/bash

# simple script to fetch last version of `replication-adapter-lib` repo
# and correctly substitute hashes in `nil.nix`

green_col=$(tput setaf 2)
reset_col=$(tput sgr0)

sed -i '' 's/^\([^a-z]*vendorHash = \)".*";/\1"";/g' nix/nil.nix

echo "calculating vendor hash for go pkgs in nix..."
nix develop --command bash -c 'GOPROXY= go mod tidy'
VENDOR_HASH=$(nix build 2>&1 | grep "got:" | awk '{ print $2; }')
echo "${green_col}vendorHash = $VENDOR_HASH${reset_col}"
VENDOR_HASH=$(echo $VENDOR_HASH 2>&1 | sed -e 's/\//\\\//g')

sed -i '' 's/^\([^a-z]*vendorHash = \)".*";/\1"'$VENDOR_HASH'";/g' nix/nil.nix
