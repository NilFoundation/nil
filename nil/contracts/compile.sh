#!/bin/bash

# Compile contract according to the given compiler json file.

set -e

FILE=$1
COMPILER_JSON=$2
OUTPUT_DIR=$3

SOLC_VER=$(jq -r '.compilerVersion' $COMPILER_JSON)

OPTIMIZE=$(jq -r '.settings.optimizer.enabled' $COMPILER_JSON)
if [ "$OPTIMIZE" == "true" ]; then
    OPTIMIZE="--optimize"
else
    OPTIMIZE=""
fi

OPTIMIZE_RUNS=$(jq -r '.settings.optimizer.runs' $COMPILER_JSON)
if [ "$OPTIMIZE_RUNS" == "null" ]; then
    OPTIMIZE_RUNS=""
else
    OPTIMIZE_RUNS="--optimize-runs $OPTIMIZE_RUNS"
fi

set +e
solc-select versions | grep $SOLC_VER &>/dev/null
SOLC_FOUND=$?
set -e

if [ $SOLC_FOUND -ne 0 ]; then
    solc-select install $SOLC_VER
fi

solc-select use $SOLC_VER &>/dev/null

solc --overwrite --metadata-hash none --abi --bin --no-cbor-metadata $OPTIMIZE $OPTIMIZE_RUNS -o $OUTPUT_DIR $FILE
