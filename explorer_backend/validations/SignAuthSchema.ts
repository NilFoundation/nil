import { z } from "zod";

export const SignAuthSchema = z.object({
  id: z.number(),
  address: z.string(),
  message: z.string(),
  expiredAt: z.number(),
  used: z.boolean(),
});
