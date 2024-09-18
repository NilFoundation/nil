#!/bin/sh -ef
mkdir -p shard0 shard1 shard2
PORTS=(31337 31338 31339)

for SH in 0 1 2; do
    pushd shard$SH
    ../build/bin/nild run --nshards 3 --my-shard $SH --http-port ${PORTS[$SH]} &
    popd
done

wait
