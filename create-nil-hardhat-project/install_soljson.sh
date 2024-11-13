#!/usr/bin/env bash
set -e

install() {
    CACHE_DIR=$1
    PLATFORM=$2
    shift 2
    DIR=$CACHE_DIR/hardhat-nodejs/compilers-v2

    PLATFORM_DIR=$DIR/$PLATFORM
    mkdir -p "$PLATFORM_DIR"
    cp list.json "$PLATFORM_DIR"
    WASM_DIR=$DIR/wasm
    mkdir -p "$WASM_DIR"
    cp list.json "$WASM_DIR"

    for var in "$@"; do
        FILE_NAME=$(basename $var)
        SOLC_NAME=$(cut -d '-' -f 2- <<<$var)
        touch "$PLATFORM_DIR"/$SOLC_NAME
        touch "$PLATFORM_DIR"/$SOLC_NAME.does.not.work
        cp "$var" "$WASM_DIR"/"$SOLC_NAME"
    done
}

# Install compiler to cache folders specific to each platform:
# https://www.npmjs.com/package/env-paths#pathscache
# This is needed to make sure that hardhat doesn't try to download anything.
# But we disable the platform's compiler and offer wasm as replacement.
# Note: linux/macosx-amd64 cover any linux/macosx, even arm64:
# https://github.com/NomicFoundation/hardhat/blob/main/packages/hardhat-core/src/internal/solidity/compiler/downloader.ts
# TODO: only install the compiler for the platform that is being tested

install "$HOME/.cache" linux-amd64 "$@"

install "$HOME/Library/Caches" macosx-amd64 "$@"
