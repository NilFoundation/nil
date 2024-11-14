import type { Abi, Address } from "abitype";
import type { XOR } from "ts-essentials";
import type { Hex } from "../types/Hex.js";
import type { Token } from "../types/Token.js";

export type SendBaseMessageParams = {
  to: Address | Uint8Array;
  refundTo?: Address | Uint8Array;
  bounceTo?: Address | Uint8Array;
  data?: Uint8Array | Hex;
  value?: bigint;
  feeCredit?: bigint;
  tokens?: Token[];
  deploy?: boolean;
  seqno?: number;
  chainId?: number;
};

export type SendDataMessageParams = SendBaseMessageParams & {
  data?: Uint8Array | Hex;
};

export type SendAbiMessageParams = SendBaseMessageParams & {
  abi: Abi;
  functionName: string;
  args?: unknown[];
};

/**
 * Represents the params for sending a message.
 *
 * @typedef {SendMessageParams}
 */
export type SendMessageParams = XOR<SendDataMessageParams, SendAbiMessageParams>;

export interface WalletInterface {
  sendMessage({
    to,
    refundTo,
    bounceTo,
    data,
    abi,
    functionName,
    args,
    deploy,
    seqno,
    feeCredit,
    value,
    tokens,
    chainId,
  }: SendMessageParams): Promise<Hex>;
}
