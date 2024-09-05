#!/usr/bin/env bash

# This script is used to start a NIL cluster with a given number of shards.
# Usage: ./run_cluster.sh <n_shards>
# It generates config files if they do not exist.
# (It actually checks the existence of the first config file only.
#  If the first config file exists, but some don't, nild will fail to start.)
# The following paths are created/used:
# - config_<n_shards>_<i>.yaml
# - main-keys_<n_shards>_<i>.yaml
# - network-keys_<n_shards>_<i>.yaml
# - nild_<n_shards>_<i>.log
# - test.db_<n_shards>_<i> directory
#
# Note for testing:
# You can have several clusters in a single working directory as long as the number of shards is different.

set -e

trap_with_arg() {
    local func="$1"
    shift
    for sig in "$@"; do
        trap "$func $sig" "$sig"
    done
}

stop() {
    trap - SIGINT EXIT
    printf '\n%s\n' "received $1, killing child processes"
    kill -s SIGINT $(jobs -pr)
}

trap_with_arg 'stop' EXIT SIGINT SIGTERM SIGHUP

DIR=$(pwd)
N_SHARDS=$1

if [ -z "$N_SHARDS" ]; then
    echo "Usage: $0 <n_shards>"
    exit 1
fi

if [ ! -f "config_""$N_SHARDS""_0.yaml" ]; then
    echo "Generating config files..."
    (
        cd "$(dirname "${BASH_SOURCE[0]}")"
        go run ../nil/tools/confgen/main.go --dir "$DIR" --n "$N_SHARDS"
    )
fi

echo "Starting $N_SHARDS shards..."

for i in $(seq 0 $((N_SHARDS - 1))); do
    nild -c "config_""$N_SHARDS""_$i.yaml" run >"nild_""$N_SHARDS""_$i.log" 2>&1 &
done

tail -f -n +1 nild_"$N_SHARDS"_*.log
