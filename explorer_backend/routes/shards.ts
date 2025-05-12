import { router, publicProcedure } from "../trpc";
import z from "zod";
import { ShardInfoSchema, getShardStats } from "../daos/shards";
import { CacheType, getCacheWithSetter } from "../services/cache";
import { fetchShardsGasPrice } from "../services/rpc";

export const shardsRouter = router({
  shardsStat: publicProcedure
    .output(z.array(ShardInfoSchema))
    .query(async () => {
      const [stat] = await getCacheWithSetter(
        "shardState",
        () => {
          return getShardStats();
        },
        {
          type: CacheType.TIMER,
          time: 60000,
        }
      );
      return stat;
    }),
  shardsGasPrice: publicProcedure
    .output(z.record(z.bigint()))
    .query(async () => {
      try {
        const gasPricemap = await fetchShardsGasPrice();
        return gasPricemap;
      } catch (e) {
        throw new Error("Failed to fetch shards gas price", { cause: e });
      }
    }),
});
