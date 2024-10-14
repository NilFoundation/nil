import { client } from "./client";

export const fetchShards = async () => {
  const res = await client.shards.shardsStat.query();
  return res;
};
