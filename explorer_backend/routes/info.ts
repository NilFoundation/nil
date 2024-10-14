import { getTransactionStat } from "../daos/transactionStat";
import { router, publicProcedure } from "../trpc";
import z from "zod";
import { TransactionStatPeriodShema, TransactionStatSchema } from "../validations/TransactionStat";
import { CacheType, getCacheWithSetter } from "../services/cache";

export const infoRouter = router({
  transactionStat: publicProcedure
    .input(
      z.object({
        period: TransactionStatPeriodShema,
      }),
    )
    .output(z.array(TransactionStatSchema))
    .query(async (opts) => {
      const { period } = opts.input;
      const [stat] = await getCacheWithSetter(
        `transactionStat:${period}`,
        () => {
          return getTransactionStat(opts.input.period);
        },
        {
          type: CacheType.TIMER,
          time: 60000,
        },
      );
      return stat;
    }),
});
