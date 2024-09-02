import type { Hex } from "./Hex.js";

/**
 * The structure representing a JSON-RPC message.
 *
 * @export
 * @typedef {RPCMessage}
 */
export type RPCMessage = {
  success: boolean;
  data: Hex;
  blockHash: Hex;
  blockNumber: number;
  from: Hex;
  gasUsed: Hex;
  gasLimit: string;
  hash: Hex;
  seqno: Hex;
  to: Hex;
  refundTo: Hex;
  bounceTo: Hex;
  index?: Hex;
  value: string;
  signature: Hex;
};
