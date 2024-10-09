import { z } from "zod";

export const TransactionStatPeriodShema = z.union([
  z.literal("1d"),
  z.literal("1m"),
  z.literal("15m"),
  z.literal("30m"),
]);

export type TransactionStatPeriod = z.infer<typeof TransactionStatPeriodShema>;

export const TransactionStatSchema = z.object({
  time: z.number(),
  value: z.number(),
  earliest_block: z.number(),
});

export type TransactionStat = z.infer<typeof TransactionStatSchema>;
