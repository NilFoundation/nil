import { z } from "zod";
import { router, publicProcedure } from "../trpc";
import { getScript, setScript } from "../services/script";

export const scriptRouter = router({
  get: publicProcedure
    .input(z.string())
    .output(
      z.object({
        script: z.string(),
      }),
    )
    .query(async (opts) => {
      const script = await getScript(opts.input as string);
      if (script === null) {
        throw new Error("Script not found");
      }
      return {
        script: script,
      };
    }),
  set: publicProcedure
    .input(z.string())
    .output(
      z.object({
        hash: z.string(),
      }),
    )
    .mutation(async (opts) => {
      const hash = setScript(opts.input);
      return {
        hash,
      };
    }),
});
