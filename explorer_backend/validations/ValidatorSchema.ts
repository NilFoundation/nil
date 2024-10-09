import { z } from "zod";

export const ValidatorSchema = z.object({
  publicKey: z.string(),
  stake: z.string(),
});
