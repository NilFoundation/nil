import type { Abi, Address } from "abitype";
import type { ISigner, PublicClient } from "../../../index.js";
import type { Token } from "../../../types/Token.js";
import type { Hex } from "../../../types/index.js";
import type {XOR} from "ts-essentials";

type WaletV1BaseConfig = {
  pubkey: Uint8Array | Hex;
  client: PublicClient;
  signer: ISigner;
};

type WalletV1ConfigCalculated = WaletV1BaseConfig & {
  salt: Uint8Array | bigint;
  shardId: number;
  address?: undefined;
};

type WalletV1ConfigAddress = WaletV1BaseConfig & {
  address: Address | Uint8Array;
  salt?: undefined;
  shardId?: undefined;
};
/**
 * Represents the wallet configuration.
 *
 
 * @typedef {WalletV1Config}
 */
export type WalletV1Config = WalletV1ConfigCalculated | WalletV1ConfigAddress;
/**
 * Represents the message call params.
 *
 
 * @typedef {CallParams}
 */
export type CallParams = {
  to: Address;
  data: Uint8Array;
  value: bigint;
};

export type SendBaseMessageParams = {
  to: Address | Uint8Array;
  refundTo?: Address | Uint8Array;
  bounceTo?: Address | Uint8Array;
  data?: Uint8Array | Hex;
  value?: bigint;
  feeCredit: bigint;
  tokens?: Token[];
  deploy?: boolean;
  seqno?: number;
  chainId?: number;
};

export type SendDataMessageParams = SendBaseMessageParams & {
  data?: Uint8Array | Hex;
}

export type SendAbiMessageParams = SendBaseMessageParams & {
  abi: Abi;
  functionName: string;
  args?: unknown[];
}

/**
 * Represents the params for sending a message.
 *
 * @typedef {SendMessageParams}
 */
export type SendMessageParams = XOR<SendDataMessageParams, SendAbiMessageParams>



export type SendSyncBaseMessageParams = {
  to: Address | Uint8Array;
  value: bigint;
  gas: bigint;
  seqno?: number;
};

export type SendSyncDataMessageParams = SendSyncBaseMessageParams & {
  data?: Uint8Array | Hex;
}

export type SendSyncAbiMessageParams = SendSyncBaseMessageParams & {
  abi: Abi;
  functionName: string;
  args?: unknown[];
}

/**
 * Represents the params for sending a message synchronously.
 *
 * @typedef {SendSyncMessageParams}
 */
export type SendSyncMessageParams = XOR<SendSyncDataMessageParams, SendSyncAbiMessageParams>


/**
 * Represents the params for making a request to the wallet.
 *
 * @typedef {RequestParams}
 */
export type RequestParams = {
  data: Uint8Array;
  deploy: boolean;
  seqno?: number;
  chainId?: number;
};

/**
 * Represents the params for deploying a smart contract.
 *
 * @typedef {DeployParams}
 */
export type DeployParams = {
  bytecode: Uint8Array | Hex;
  abi?: Abi;
  args?: unknown[];
  salt: Uint8Array | bigint;
  shardId: number;
  feeCredit: bigint;
  value?: bigint;
  seqno?: number;
  chainId?: number;
};
