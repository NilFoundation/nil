import type { Abi, Address } from "abitype";
import type { Hex } from "./Hex.js";
import type { XOR } from "ts-essentials";

/**
 * The args for performing a call to a smart contract.
 *
 * @export
 * @typedef {BaseCallArgs}
 */
export type BaseCallArgs = {
  from?: Address;
  to: Address;
  feeCredit?: bigint;
  value?: bigint;
};

/**
 * The args for performing a call with some embedded data.
 *
 * @export
 * @typedef {DataCallArgs}
 */
export type DataCallArgs = BaseCallArgs & {
  data: Uint8Array | Hex;
};

/**
 * The args for peforming a call with the specified ABI and function name.
 *
 * @export
 * @typedef {AbiCallArgs}
 */
export type AbiCallArgs = BaseCallArgs & {
  abi: Abi;
  functionName: string;
  args?: unknown[];
};

/**
 * The args for performing either a call with embedded data or a call with a specific ABI and function name.
 *
 * @export
 * @typedef {CallArgs}
 */
export type CallArgs = XOR<DataCallArgs, AbiCallArgs>;

/**
 * The structure for overriding certain contract params.
 *
 * @export
 * @typedef {ContractOverride}
 */
export type ContractOverride = {
  balance?: bigint;
  code?: Uint8Array | Hex;
  seqno?: bigint;
  extSeqno?: bigint;
  state?: unknown;
  stateDiff?: unknown;
};

/**
 * The structure representing an outgoing message (a message created by a smart contract after another message triggers its execution).
 *
 * @export
 * @typedef {OutMessage}
 */
export type OutMessage = {
  from?: Address;
  to?: Address;
  feeCredit?: string;
  seqno: string;
  data?: Uint8Array;
  forwardFee?: string;
  coinsUsed: string;
  outMessages: OutMessage[];
  error?: string;
};

/**
 * The structure representing the results of a call.
 *
 * @export
 * @typedef {CallRes}
 */
export type CallRes = {
  data: `0x${string}`;
  decodedData?: unknown;
  coinsUsed: string;
  outMessages: OutMessage[];
};
