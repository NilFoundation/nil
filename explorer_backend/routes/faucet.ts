import { z } from "zod";
import { router, publicProcedure } from "../trpc";

export const faucetRouter = router({
  transfer: publicProcedure
    .input(
      z.object({
        address: z.string(),
        value: z.string(),
      }),
    )
    .output(
      z.object({
        txHash: z.string(),
        mined: z.boolean(),
      }),
    )
    .mutation(async (_opts) => {
      throw new Error("Not implemented");
    }),
});
