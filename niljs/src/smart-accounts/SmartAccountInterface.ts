import type { Abi, Address } from "abitype";
import type { XOR } from "ts-essentials";
import type { Hex } from "../types/Hex.js";
import type { Token } from "../types/Token.js";
import type { Transaction } from "../utils/transaction.js";
import type { DeployParams } from "./SmartAccountV1/types.js";

export type SendBaseTransactionParams = {
  to: Address | Uint8Array;
  refundTo?: Address | Uint8Array;
  bounceTo?: Address | Uint8Array;
  data?: Uint8Array | Hex;
  value?: bigint;
  feeCredit?: bigint;
  maxPriorityFeePerGas?: bigint;
  maxFeePerGas?: bigint;
  tokens?: Token[];
  deploy?: boolean;
  seqno?: number;
  chainId?: number;
};

export type SendDataTransactionParams = SendBaseTransactionParams & {
  data?: Uint8Array | Hex;
};

export type SendAbiTransactionParams = SendBaseTransactionParams & {
  abi: Abi;
  functionName: string;
  args?: unknown[];
};

/**
 * Represents the params for sending a transaction.
 *
 * @typedef {SendTransactionParams}
 */
export type SendTransactionParams = XOR<SendDataTransactionParams, SendAbiTransactionParams>;

export interface SmartAccountInterface {
  sendTransaction({
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
  }: SendTransactionParams): Promise<Transaction>;

  deployContract({
                   shardId,
                   bytecode,
                   abi,
                   args,
                   salt,
                   value,
                   feeCredit,
                   maxPriorityFeePerGas,
                   maxFeePerGas,
                   seqno,
                   chainId,
                 }: DeployParams
  ): Promise<{ tx: Transaction; address: Hex }>;

  shardId: number;
  address: Address;
  getBalance(): Promise<bigint>;
}
