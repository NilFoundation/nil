#!/usr/bin/env bash
set -e

for platform in linux-amd64 windows-amd64 macosx-amd64 wasm; do
	mkdir -p ~/.cache/hardhat-nodejs/compilers-v2/$platform
	cp list.json ~/.cache/hardhat-nodejs/compilers-v2/$platform
	cp "$1" ~/.cache/hardhat-nodejs/compilers-v2/$platform/soljson-v0.8.26+commit.8a97fa7a.js
	if [ "$platform" != "wasm" ]; then
		touch ~/.cache/hardhat-nodejs/compilers-v2/$platform/soljson-v0.8.26+commit.8a97fa7a.js.does.not.work
	fi
done
