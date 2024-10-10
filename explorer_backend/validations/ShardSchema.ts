import { z } from "zod";

export type ShardType = {
  shard: string;
};

export const ShardSchema = z.object({
  shard: z.string(),
});
