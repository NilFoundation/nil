import { createDomain } from "effector";
import { fetchShards, fetchShardsGasPrice } from "../../../api/shards";

type Shards = Awaited<ReturnType<typeof fetchShards>>;

export const explorerShardsList = createDomain("shards-list");

const createStore = explorerShardsList.createStore.bind(explorerShardsList);
const createEffect = explorerShardsList.createEffect.bind(explorerShardsList);

export const $shards = createStore<Shards>([]);

export const $totalTxOnShards = $shards.map((shards) =>
  shards.reduce((acc, shard) => acc + shard.tx_count, 0),
);

export const $shardsGasPriceMap = explorerShardsList.createStore<Record<string, bigint>>({});

const $shardsGasPriceMapWithoutMain = $shardsGasPriceMap.map((shards) => {
  const newShards = { ...shards };
  // biome-ignore lint/performance/noDelete: it des not affect performance for objects.
  delete newShards[0];
  return newShards;
});

export const $busiestShard = $shardsGasPriceMapWithoutMain.map((shardsGasPriceMap) => {
  const busiestShard = Object.entries(shardsGasPriceMap).reduce(
    (max, [shardId, gasPrice]) => (gasPrice > max[1] ? [shardId, gasPrice] : max),
    ["", BigInt(0)],
  );
  return busiestShard;
});

export const $bestShard = $shardsGasPriceMapWithoutMain.map((shardsGasPriceMap) => {
  const bestShard = Object.entries(shardsGasPriceMap).reduce(
    (min, [shardId, gasPrice]) => (gasPrice < min[1] ? [shardId, gasPrice] : min),
    ["", BigInt(Number.MAX_SAFE_INTEGER)],
  );
  return bestShard;
});

export const fetchShardsFx = createEffect<void, Shards, Error>();

fetchShardsFx.use(fetchShards);

export const fetchShrdsGasPriceFx = createEffect<void, Record<string, bigint>, Error>();

fetchShrdsGasPriceFx.use(async () => {
  const gasPriceMap = await fetchShardsGasPrice();
  return Object.fromEntries(
    Object.entries(gasPriceMap).map(([shardId, gasPrice]) => [shardId, BigInt(gasPrice)]),
  );
});
