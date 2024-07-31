import type { Address } from "abitype";
import type { Hex } from "./Hex.js";

export type ProcessedMessage = {
  success: boolean;
  data: Hex;
  blockHash: Hex;
  blockNumber: number;
  from: Address;
  gasUsed: bigint;
  gasLimit: bigint;
  hash: Hex;
  seqno: bigint;
  to: Address;
  refundTo: Address;
  bounceTo: Address;
  index?: number;
  value: bigint;
  signature: Hex;
};
