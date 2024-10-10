import { removeHexPrefix } from "@nilfoundation/niljs";
import { client } from "../services/clickhouse";
import { z } from "zod";

const fields = `shard_id, 
hex(hash) AS hash,
hex(prev_block) as prev_block,
hex(main_chain_hash) as master_chain_hash,
out_msg_num,
in_msg_num,
timestamp,
id`;

export type BlockListElement = {
  shard_id: number;
  hash: string;
  prev_block: string;
  master_chain_hash: string;
  out_msg_num: string;
  in_msg_num: string;
  timestamp: string;
  id: string;
};

export const BlockListElementScheme = z.object({
  shard_id: z.number(),
  hash: z.string(),
  prev_block: z.string(),
  master_chain_hash: z.string(),
  out_msg_num: z.string(),
  in_msg_num: z.string(),
  timestamp: z.string(),
  id: z.string(),
});

export const fetchLatestBlocks = async (offset: number, limit: number): Promise<BlockListElement[]> => {
  const query = await client.query({
    query: `SELECT
    ${fields}
     FROM blocks ORDER BY id DESC LIMIT {limit: Int32} OFFSET {offset: Int32}`,
    query_params: {
      offset,
      limit,
    },
    format: "JSON",
  });
  try {
    const res = await query.json<BlockListElement>();
    return res.data;
  } finally {
    query.close();
  }
};

export const fetchBlockByHash = async (hash: string): Promise<BlockListElement | null> => {
  const query = await client.query({
    query: `SELECT ${fields} FROM blocks WHERE hash = {hash: String} limit 1`,
    query_params: {
      hash: removeHexPrefix(hash).toUpperCase(),
    },
    format: "JSON",
  });

  try {
    const res = await query.json<BlockListElement>();
    if (res.data.length === 0) return null;
    return res.data[0];
  } finally {
    query.close();
  }
};

export const fetchBlocksByShardAndNumber = async (shardId: number, seqNo: number): Promise<BlockListElement | null> => {
  const query = await client.query({
    query: `SELECT ${fields} FROM blocks WHERE shard_id = {shardId: Int32} AND id = {seqNo: Int32}`,
    query_params: {
      shardId,
      seqNo,
    },
    format: "JSON",
  });
  try {
    const res = await query.json<BlockListElement>();
    if (res.data.length === 0) return null;
    return res.data[0];
  } finally {
    query.close();
  }
};
