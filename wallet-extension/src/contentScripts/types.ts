import { z } from "zod";
import type { BaseEthereumRequest } from "./WindowNilRequestTypes.ts";

/* eslint-disable no-restricted-syntax  */
const ExtensionResponseSchema = z
  .object({
    requestId: z.string(),
    result: z.any().optional(),
    error: z.any().optional(),
  })
  .refine((data) => data.result !== undefined || data.error !== undefined, {
    message: "Either result or error must be defined",
  });

export type ExtensionResponse = z.infer<typeof ExtensionResponseSchema>;

export const isValidExtensionResponse = (response: unknown): response is ExtensionResponse => {
  return ExtensionResponseSchema.safeParse(response).success;
};

export const WindowNilRequestSchema = z.object({
  method: z.string(),
  params: z.any().optional(),
  requestId: z.string(),
  origin: z.string().optional(),
});
export type WindowNilRequest = z.infer<typeof WindowNilRequestSchema>;

export const isValidWindowNilRequest = (request: unknown): request is WindowNilRequest => {
  return WindowNilRequestSchema.safeParse(request).success;
};

export type RequestInput = BaseEthereumRequest & { id?: number; jsonrpc?: string };

// Keep this type to be compatible with deprecated EIP-1193 method
export type EthersSendCallback = (error: unknown, response: unknown) => void;
