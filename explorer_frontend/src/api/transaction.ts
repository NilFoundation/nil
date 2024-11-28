import type { ProcedureOptions } from "@trpc/server";
import type { TimeInterval } from "../features/transaction-stat";
import { client } from "./client";

export const fetchTransactionByHash = async (hash: string, opts: ProcedureOptions) => {
  const res = await client.transactions.transactionByHash.query(hash, opts);
  return res;
};

export const fetchLatestBlocks = async () => {
  const res = await client.block.latestBlocks.query();
  return res;
};

export const fetchTransactionsByBlockHash = async (hash: string) => {
  const res = await client.transactions.transactionsByBlockHash.query(hash);
  return res;
};

export const fetchTransactionsByBlock = async (shard: string, id: string) => {
  const res = await client.transactions.transactionsByBlock.query({
    shard: +shard,
    seqno: +id,
  });
  return res;
};

export const fetchTransactionsByAddress = async (address: string) => {
  const res = await client.transactions.transactionsByAddress.query({
    address,
    offset: 0,
    limit: 100,
  });
  return res;
};

export const getTransactionStat = async (interval: TimeInterval) => {
  const res = await client.info.transactionStat.query({ period: interval });
  return res;
};

export const getChildTransactionsByHash = async (hash: string) => {
  const res = await client.transactions.getChildTransactionsByHash.query(hash);
  return res;
};

export const getTransactionLogsByHash = async (hash: string) => {
  const res = await client.transactions.getTransactionLogsByHash.query(hash);
  return res;
};
