import { client } from "./client";

export const fetchLatestBlocks = async () => {
  const res = await client.block.latestBlocks.query();
  return res;
};

export const fetchBlockByHash = async (hash: string) => {
  const res = await client.block.blockByHash.query(hash);
  return res;
};

export const fetchBlock = async (shard: number, id: string) => {
  const res = await client.block.block.query({
    shard,
    seqno: +id,
  });
  return res;
};
