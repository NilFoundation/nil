import { z } from "zod";
import { router, publicProcedure } from "../trpc";
import {
  type BlockListElement,
  BlockListElementScheme,
  fetchBlockByHash,
  fetchBlocksByShardAndNumber,
  fetchLatestBlocks,
} from "../daos/blocks";
import { CacheType, getCacheWithSetter } from "../services/cache";

export const blockRouter = router({
  latestBlocks: publicProcedure.output(z.array(BlockListElementScheme)).query(async () => {
    const [blocks] = await getCacheWithSetter<BlockListElement[]>(
      "latestBlocks",
      () => {
        return fetchLatestBlocks(0, 10);
      },
      {
        type: CacheType.TIMER,
        time: 5000,
      },
    );

    return blocks;
  }),
  block: publicProcedure
    .input(
      z.object({
        seqno: z.number(),
        shard: z.number(),
      }),
    )
    .output(BlockListElementScheme)
    .query(async (opts) => {
      const { seqno, shard } = opts.input;
      const block = await fetchBlocksByShardAndNumber(shard, seqno);
      if (!block) {
        throw new Error("Block not found");
      }
      return block;
    }),
  blockByHash: publicProcedure
    .input(z.string())
    .output(BlockListElementScheme)
    .query(async (opts) => {
      const block = await fetchBlockByHash(opts.input);
      if (!block) {
        throw new Error("Block not found");
      }
      return block;
    }),
});
