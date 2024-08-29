import type {Abi, Address} from "abitype";
import type { Hex } from "./Hex.js";
import type { XOR } from "ts-essentials";

export type BaseCallArgs = {
  from?: Address;
  to: Address;
  feeCredit?: bigint;
  value?: bigint;
};

export type DataCallArgs = BaseCallArgs & {
    data: Uint8Array | Hex;
};

export type AbiCallArgs = BaseCallArgs & {
  abi: Abi;
  functionName: string;
  args?: unknown[];
}

export type CallArgs = XOR<DataCallArgs, AbiCallArgs>;

export type ContractOverride = {
  balance?: bigint;
  code?: Uint8Array | Hex;
  seqno?: bigint;
  extSeqno?: bigint;
  state?: unknown;
  stateDiff?: unknown;
}

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
}

export type CallRes = {
  data: `0x${string}`;
  decodedData?: unknown;
  coinsUsed: string;
  outMessages: OutMessage[];
}