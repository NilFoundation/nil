#!/usr/bin/env bash
set -e

FILE_NAME="soljson-v0.8.26+commit.8a97fa7a.js"

install() {
    CACHE_DIR=$1
    PLATFORM=$2
    DIR=$CACHE_DIR/hardhat-nodejs/compilers-v2

    PLATFORM_DIR=$DIR/$PLATFORM
    mkdir -p "$PLATFORM_DIR"
    cp list.json "$PLATFORM_DIR"
    touch "$PLATFORM_DIR"/$FILE_NAME
    touch "$PLATFORM_DIR"/$FILE_NAME.does.not.work

    WASM_DIR=$DIR/wasm
    mkdir -p "$WASM_DIR"
    cp list.json "$WASM_DIR"
    cp "$3" "$WASM_DIR"/$FILE_NAME
}

SRC=$1

# Install compiler to cache folders specific to each platform:
# https://www.npmjs.com/package/env-paths#pathscache
# This is needed to make sure that hardhat doesn't try to download anything.
# But we disable the platform's compiler and offer wasm as replacement.
# Note: linux/macosx-amd64 cover any linux/macosx, even arm64:
# https://github.com/NomicFoundation/hardhat/blob/main/packages/hardhat-core/src/internal/solidity/compiler/downloader.ts
# TODO: only install the compiler for the platform that is being tested

install "$HOME/.cache" linux-amd64 "$SRC"

install "$HOME/Library/Caches" macosx-amd64 "$SRC"
