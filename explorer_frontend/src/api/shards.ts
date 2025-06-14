import { client } from "./client";

export const fetchShards = async () => {
  const res = await client.shards.shardsStat.query();
  return res;
};

export const fetchShardsGasPrice = async () => {
  const res = await client.shards.shardsGasPrice.query();
  return res;
};
