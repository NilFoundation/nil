import type { IAddress } from "../signers/types/IAddress.js";
import type { Hex } from "../types/Hex.js";
import { BaseClient } from "./BaseClient.js";

export class IndexerClient extends BaseClient {
  /**
   * Gets address actions page
   * @param address - The address to get actions for.
   * @param sinceBlockNumber - The timestamp to get actions since.
   * @returns The page of address actions.
   */
  public async getAddressActions(address: Hex, sinceBlockNumber = 0) {
    return await this.request<AddressAction[]>({
      method: "indexer_getAddressActions",
      params: [address, sinceBlockNumber],
    });
  }
}

export type AddressAction = {
  hash: Hex;
  from: IAddress;
  to: IAddress;
  amount: bigint;
  blockId: number;
  type: AddressActionKind;
  status: AddressActionStatus;
};

export enum AddressActionKind {
  SendEth = "SendEth",
  ReceiveEth = "ReceiveEth",
  SmartContractCall = "SmartContractCall",
}

export enum AddressActionStatus {
  Success = "Success",
  Failed = "Failed",
}
