import type { Address } from "abitype";
import type { Hex } from "./Hex.js";

export type CallArgs = {
  from: Address;
  to: Address;
  gasLimit?: bigint;
  value?: bigint;
  data: Uint8Array | Hex;
};
