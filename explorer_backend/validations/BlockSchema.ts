import { z } from "zod";

export const BlockSchema = z.object({
  shard_id: z.number(),
  prev_block: z.string(),
  master_chain_hash: z.string(),
  hash: z.string(),
  out_transaction_num: z.number(),
  id: z.string(),
});
