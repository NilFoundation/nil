import { z } from "zod";
import { router, publicProcedure } from "../trpc";
import { getCode, setCode } from "../services/sqlite";

export const codeRouter = router({
  get: publicProcedure
    .input(z.string())
    .output(
      z.object({
        code: z.string(),
      }),
    )
    .query(async (opts) => {
      const code = await getCode(opts.input as string);
      if (code === null) {
        throw new Error("Code not found");
      }
      return {
        code,
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
      const hash = await setCode(opts.input);
      return {
        hash,
      };
    }),
});
