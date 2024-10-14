import { z } from "zod";

export const TransactionLogSchema = z.object({
  message_hash: z.string(),
  address: z.string(),
  topics_count: z.number(),
  topic1: z.string(),
  topic2: z.string(),
  topic3: z.string(),
  topic4: z.string(),
  data: z.string(),
});

export type TransactionLog = z.infer<typeof TransactionLogSchema>;
