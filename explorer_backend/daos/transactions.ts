import { client } from "../services/clickhouse";
import { z } from "zod";
import type { TransactionLog } from "../validations/TransactionLog";
import { hexToBigInt, numberToHex } from "viem";
import { addHexPrefix, removeHexPrefix } from "@nilfoundation/niljs";

export type ClickhouseTransactionElem = {
  hash: string;
  block_hash: string;
  from: string;
  to: string;
  shard_id: number;
  block_id: string;
  success: boolean;
  gas_used: string;
  fee_credit: string;
  seqno: string;
  value: string;
  method: string;
  flags: number;
  timestamp: string;
  outgoing: boolean;
  token: [string, string][];
};

export const TokenSchema = z.object({
  token: z.string(),
  balance: z.string(),
});

export type Token = z.infer<typeof TokenSchema>;

export const TransactionElemSchema = z.object({
  hash: z.string(),
  block_hash: z.string(),
  from: z.string(),
  to: z.string(),
  shard_id: z.number(),
  block_id: z.number(),
  success: z.boolean(),
  gas_used: z.number(),
  fee_credit: z.string(),
  seqno: z.number(),
  value: z.string(),
  method: z.string(),
  flags: z.number(),
  timestamp: z.string(),
  outgoing: z.boolean(),
});

export const TransactionFullSchema = z.object({
  hash: z.string(),
  block_hash: z.string(),
  from: z.string(),
  to: z.string(),
  shard_id: z.number(),
  block_id: z.number(),
  success: z.boolean(),
  gas_used: z.number(),
  fee_credit: z.string(),
  seqno: z.number(),
  value: z.string(),
  method: z.string(),
  flags: z.number(),
  timestamp: z.string(),
  outgoing: z.boolean(),
  token: z.array(TokenSchema),
});

export type TransactionElem = z.infer<typeof TransactionElemSchema>;

export type TransactionFull = z.infer<typeof TransactionFullSchema>;

const fieldsElem = `hex(hash) as hash,
hex(block_hash) as block_hash,
hex(from) as from,
hex(to) as to,
shard_id,
block_id,
success,
outgoing,
flags,
gas_used,
seqno,
value,
fee_credit,
timestamp,
substring(arrayStringConcat(arrayMap(x -> hex(x), data)), 1, 8) as method`;

const fieldsFull = `hex(hash) as hash,
hex(block_hash) as block_hash,
hex(from) as from,
hex(to) as to,
shard_id,
block_id,
success,
outgoing,
flags,
gas_used,
seqno,
value,
fee_credit,
timestamp,
arrayStringConcat(arrayMap(x -> hex(x), data)) as method,
arrayMap(x -> tuple(hex(tupleElement(x, 1)), tupleElement(x, 2)), token) as token`;

const clickhouseTransactionUnwrap = (elem: ClickhouseTransactionElem): TransactionElem => {
  return {
    ...elem,
    block_id: Number.parseInt(elem.block_id, 10),
    gas_used: Number.parseInt(elem.gas_used, 10),
    seqno: Number.parseInt(elem.seqno, 10),
  };
};

export const getTransactionsByBlockHash = async (hash: string): Promise<TransactionElem[]> => {
  const query = await client.query({
    query: `SELECT
        ${fieldsElem}
        FROM transactions WHERE block_hash = {hash: String}
        ORDER BY
          timestamp DESC,
          block_id DESC,
          transaction_index ASC,
          seqno DESC,
          outgoing ASC,
        `,
    query_params: {
      hash,
    },
    format: "JSON",
  });
  try {
    const res = await query.json<ClickhouseTransactionElem>();
    return res.data.map(clickhouseTransactionUnwrap);
  } finally {
    await query.close();
  }
};
export const getTransactionsByBlock = async (
  shard: number,
  id: number,
): Promise<TransactionElem[]> => {
  const query = await client.query({
    query: `SELECT
        ${fieldsElem}
        FROM transactions WHERE block_id = {id: Int32} AND shard_id = {shard: Int32}
        ORDER BY
          timestamp DESC,
          block_id DESC,
          transaction_index ASC,
          seqno DESC,
          outgoing ASC,
        `,
    query_params: {
      shard,
      id,
    },
    format: "JSON",
  });
  try {
    const res = await query.json<ClickhouseTransactionElem>();
    return res.data.map(clickhouseTransactionUnwrap);
  } finally {
    await query.close();
  }
};

export const getTransactionsByAddress = async (address: string, offset: number, limit: number) => {
  if (offset > 1000) {
    throw new Error("Offset is too large");
  }
  if (limit > 100) {
    throw new Error("Limit is too large");
  }
  console.log(`SELECT
        ${fieldsElem}
        FROM transactions
        WHERE from = {address: String} OR to = {address: String} ORDER BY timestamp DESC LIMIT {limit: Int32} OFFSET {offset: Int32}
        `);
  const query = await client.query({
    query: `SELECT
        ${fieldsElem}
        FROM transactions
        WHERE from = {address: String} OR to = {address: String}
        ORDER BY
          timestamp DESC,
          block_id DESC,
          seqno DESC,
          outgoing ASC
        LIMIT {limit: Int32} OFFSET {offset: Int32}
        `,
    query_params: {
      address: address.slice(2).toUpperCase(),
      offset,
      limit,
    },
    format: "JSON",
  });
  try {
    const res = await query.json<ClickhouseTransactionElem>();
    return res.data.map(clickhouseTransactionUnwrap);
  } finally {
    await query.close();
  }
};

export const getTransactionByHash = async (hash: string): Promise<TransactionFull | null> => {
  const query = await client.query({
    query: `SELECT
        ${fieldsFull}
        FROM transactions WHERE hash = {hash: String} and outgoing = false
        LIMIT 1
        `,
    query_params: {
      hash: hash.toUpperCase(),
    },
    format: "JSON",
  });
  try {
    const res = await query.json<ClickhouseTransactionElem>();
    if (res.data.length === 0) {
      return null;
    }
    const tokens = res.data[0].token.map((c) => {
      const numToken = hexToBigInt(addHexPrefix(c[0]));
      const address = numberToHex(numToken, {
        size: 20,
      });
      return TokenSchema.parse({ token: address, balance: c[1] });
    });
    const t = {
      ...clickhouseTransactionUnwrap(res.data[0]),
      token: tokens,
    };
    return t;
  } finally {
    query.close();
  }
};

export const getChildTransactionsByHash = async (hash: string) => {
  const query = await client.query({
    query: `SELECT
    hex(incoming.hash) as hash,
    hex(incoming.block_hash) as block_hash,
    hex(incoming.from) as from,
    hex(incoming.to) as to,
    incoming.shard_id as shard_id,
    incoming.block_id as block_id,
    incoming.success as success,
    incoming.outgoing as outgoing,
    incoming.flags as flags,
    incoming.gas_used as gas_used,
    incoming.seqno as seqno,
    incoming.value as value,
    incoming.fee_credit as fee_credit,
    incoming.timestamp as timestamp,
    substring(arrayStringConcat(arrayMap(x -> hex(x), incoming.data)), 1, 8) as method
FROM transactions as outgoing
LEFT OUTER JOIN transactions as incoming
ON outgoing.hash = incoming.hash AND incoming.outgoing = false

WHERE outgoing.parent_transaction = unhex({hash: String}) and outgoing.outgoing = true
`,
    query_params: {
      hash: removeHexPrefix(hash),
    },
    format: "JSON",
  });
  try {
    const res = await query.json<ClickhouseTransactionElem>();
    console.log(res.data);
    return res.data.map(clickhouseTransactionUnwrap);
  } finally {
    query.close();
  }
};

export const getTransactionLogsByHash = async (hash: string) => {
  const query = await client.query({
    query: `SELECT
        hash as transaction_hash,
        address,
        topics_count,
        topic1,
        topic2,
        topic3,
        topic4,
        data
      FROM (
        SELECT
          hex(transaction_hash) as hash,
          hex(address) as address,
          topics_count,
          hex(topic1) as topic1,
          hex(topic2) as topic2,
          hex(topic3) as topic3,
          hex(topic4) as topic4,
          arrayStringConcat(arrayMap(x -> hex(x), data)) as data
        FROM logs
        WHERE transaction_hash = unhex({hash: String})
      )
        `,
    query_params: {
      hash,
    },
    format: "JSON",
  });
  try {
    return (await query.json<TransactionLog>()).data;
  } finally {
    query.close();
  }
};
