#!/bin/bash

SCRIPT_DIR=$(dirname $(realpath "$0"))

OUTDIR="${1:-.}"

CONTRACTS=(
    Faucet.sol
)

pushd ${SCRIPT_DIR}

for CONTRACT in ${CONTRACTS[*]}
do
    go run ../tools/solc/bin/main.go -s ${CONTRACT} -o "${OUTDIR}"
done
