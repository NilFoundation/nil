import { z } from "zod";
export const BaseNilRequestSchema = z.object({
  method: z.string(),
  params: z.union([z.array(z.unknown()), z.record(z.string(), z.unknown())]).optional(),
});

export type BaseEthereumRequest = z.infer<typeof BaseNilRequestSchema>;
