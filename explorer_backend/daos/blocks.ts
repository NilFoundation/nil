import { removeHexPrefix } from "@nilfoundation/niljs";
import { client } from "../services/clickhouse";
import { z } from "zod";

const formatFields = (prefix = "") => `${prefix ? `${prefix}.` : ""}shard_id as shard_id,
hex(${prefix ? `${prefix}.` : ""}hash) AS hash,
hex(${prefix ? `${prefix}.` : ""}prev_block) as prev_block,
hex(${prefix ? `${prefix}.` : ""}main_chain_hash) as master_chain_hash,
${prefix ? `${prefix}.` : ""}out_transaction_num as out_txn_num,
${prefix ? `${prefix}.` : ""}in_txn_num as in_txn_num,
${prefix ? `${prefix}.` : ""}timestamp as timestamp,
${prefix ? `${prefix}.` : ""}id as id`;

export type BlockListElement = {
  shard_id: number;
  hash: string;
  prev_block: string;
  master_chain_hash: string;
  out_txn_num: string;
  in_txn_num: string;
  timestamp: string;
  id: string;
};

export const BlockListElementScheme = z.object({
  shard_id: z.number(),
  hash: z.string(),
  prev_block: z.string(),
  master_chain_hash: z.string(),
  out_txn_num: z.string(),
  in_txn_num: z.string(),
  timestamp: z.string(),
  id: z.string(),
});

export const fetchLatestBlocks = async (
  offset: number,
  limit: number,
): Promise<BlockListElement[]> => {
  const query = await client.query({
    query: `SELECT
    ${formatFields("all_blocks")}
    FROM blocks as main_blocks
    LEFT OUTER JOIN blocks as all_blocks
    ON (main_blocks.hash = all_blocks.main_chain_hash or main_blocks.hash = all_blocks.hash)
    WHERE main_blocks.shard_id = 0
    and main_blocks.id > 0 and all_blocks.id > 0
    order by main_blocks.id desc, all_blocks.id desc
    LIMIT {limit: Int32} OFFSET {offset: Int32}`,
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
    query: `SELECT ${formatFields()} FROM blocks WHERE hash = {hash: String} limit 1`,
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

export const fetchBlocksByShardAndNumber = async (
  shardId: number,
  seqNo: number,
): Promise<BlockListElement | null> => {
  const query = await client.query({
    query: `SELECT ${formatFields()} FROM blocks WHERE shard_id = {shardId: Int32} AND id = {seqNo: Int32}`,
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
