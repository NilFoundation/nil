#!/bin/sh -ef
mkdir -p shard0 shard1 shard2
PORTS=(31337 31338 31339)

ENDPOINTS="0=http://127.0.0.1:${PORTS[0]},1=http://127.0.0.1:${PORTS[1]},2=http://127.0.0.1:${PORTS[2]}"

for SH in 0 1 2; do
    pushd shard$SH
    ../build/bin/nild run --nshards 3 --my-shard $SH --port ${PORTS[$SH]} \
        --shard-endpoints "$ENDPOINTS" &
    popd
done

wait
