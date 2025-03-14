#!/bin/bash

# Set the root directory of the workspace
WORKSPACE_ROOT=$(cd "$(dirname "$0")/../.." && pwd)

# Create a temporary remappings file with absolute paths
TEMP_REMAPPINGS=$(mktemp)
sed "s|node_modules|$WORKSPACE_ROOT/nil/node_modules|g" remappings.txt > "$TEMP_REMAPPINGS"

# Read the remappings from the temporary file and format them as individual arguments
REMAPPINGS=$(awk '{print "--remappings", $0}' "$TEMP_REMAPPINGS" | tr '\n' ' ')

# Run forge with the remappings as arguments
eval forge "$@" $REMAPPINGS

# Clean up the temporary remappings file
rm "$TEMP_REMAPPINGS"
