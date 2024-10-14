import { addHexPrefix } from "@nilfoundation/niljs";
import { isAddress } from "viem";
import { z } from "zod";

export const ethAddressSchema = z.string().refine((value) => isAddress(addHexPrefix(value)), {
  message: "Provided address is invalid. Please insure you have typed correctly.",
});
