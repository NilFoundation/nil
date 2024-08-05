#!/bin/sh -efx
mkdir -p shard1 shard2
PORT1=31337
PORT2=31338

pushd shard1
../build/bin/nil run --nshards 3 --run-only-shard 1 --port $PORT1 \
    --shard-endpoints 2=http://127.0.0.1:$PORT2 &
popd

pushd shard2
../build/bin/nil run --nshards 3 --run-only-shard 2 --port $PORT2 \
    --shard-endpoints 1=http://127.0.0.1:$PORT1 &
popd

wait