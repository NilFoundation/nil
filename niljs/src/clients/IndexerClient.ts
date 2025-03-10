import type { Hex } from "../types/Hex.js";
import { BaseClient } from "./BaseClient.js";
import type { IClientBaseConfig } from "./types/Configs.js";
import type { IAddress } from "../signers/types/IAddress.js";

class IndexerClient extends BaseClient {
  // biome-ignore lint/complexity/noUselessConstructor: may be useful in the future
  constructor(config: IClientBaseConfig) {
    super(config);
  }

  /**
   * Gets address actions page
   * @param address - The address to get actions for.
   * @param sinceTimestamp - The timestamp to get actions since.
   * @returns The page of address actions.
   */
  public async getAddressActions(address: Hex, sinceTimestamp: number = 0) {
    return await this.request<AddressAction[]>({
      method: "indexer_getAddressActions",
      params: [address, sinceTimestamp],
    });
  }
}

export type AddressAction = {
  hash: Hex
  from: IAddress
  to: IAddress
  amount: bigint
  timestamp: number
  blockId: number
  type: AddressActionKind
  status: AddressActionStatus
}

export enum AddressActionKind {
  SendEth = "SendEth",
  ReceiveEth = "ReceiveEth",
  SmartContractCall = "SmartContractCall",
}

export enum AddressActionStatus {
  Success = "Success",
  Failed = "Failed",
}

export { IndexerClient };
