import { z } from "zod";
import { router, publicProcedure } from "../trpc";
import { getCode, setCode } from "../services/sqlite";

export const codeRouter = router({
  get: publicProcedure
    .input(z.string())
    .output(
      z.array(
        z.object({
          path: z.string(),
          content: z.string().nullable(),
        })
      )
    )
    .query(async (opts) => {
      const res = await getCode(opts.input as string);
      if (res === null) {
        throw new Error("Project not found");
      }
      return res;
    }),
  set: publicProcedure
    .input(
      z.object({})
    )
    .output(
      z.object({
        hash: z.string(),
      }),
    )
    .mutation(async (opts) => {
      const project = opts.input;

      const hash = await setCode(project);

      return {
        hash,
      };
    }),
});
