import { router, publicProcedure } from "../trpc";
import z from "zod";
import { ShardInfoSchema, getShardStats } from "../daos/shards";
import { CacheType, getCacheWithSetter } from "../services/cache";

export const shardsRouter = router({
  shardsStat: publicProcedure.output(z.array(ShardInfoSchema)).query(async () => {
    const [stat] = await getCacheWithSetter(
      "shardState",
      () => {
        return getShardStats();
      },
      {
        type: CacheType.TIMER,
        time: 60000,
      },
    );
    return stat;
  }),
});
