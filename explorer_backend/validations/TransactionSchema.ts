import { z } from "zod";

export const TransactionSchema = z.object({
  shard: z.string(),
  seqno: z.number(),
  payload: z.string(),
  hash: z.string(),
  account: z.string(),
  fee: z.string(),
  lt: z.string(),
  block_hash: z.string(),
});
