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

export const $busiestShard = $shardsGasPriceMap.map((shardsGasPriceMap) => {
  const maxGasPriceValue = Math.max(...Object.values(shardsGasPriceMap).map(Number));
  return Object.entries(shardsGasPriceMap).find(
    ([_, gasPrice]) => Number(gasPrice) === maxGasPriceValue,
  );
});

export const $bestShard = $shardsGasPriceMap.map((shardsGasPriceMap) => {
  const minGasPriceValue = Math.min(...Object.values(shardsGasPriceMap).map(Number));
  return Object.entries(shardsGasPriceMap).find(
    ([_, gasPrice]) => Number(gasPrice) === minGasPriceValue,
  );
});

export const fetchShardsFx = createEffect<void, Shards, Error>();

fetchShardsFx.use(fetchShards);

export const fetchShrdsGasPriceFx = createEffect<void, Record<string, bigint>, Error>();

fetchShrdsGasPriceFx.use(fetchShardsGasPrice);
