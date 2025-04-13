import type { PublicClient } from "../clients/PublicClient.js";
import { bytesToHex } from "../encoding/fromBytes.js";
import type { Hex } from "../types/Hex.js";
import type { ProcessedReceipt, TransactionOptions } from "../types/IReceipt.js";
import { waitTillCompleted } from "./receipt.js";

/**
 * Represents a transaction that can be awaited for completion
 */
export class Transaction {
  public readonly hash: Hex;

  constructor(
    private readonly txHash: Uint8Array,
    private readonly client: PublicClient,
  ) {
    this.hash = bytesToHex(this.txHash);
  }

  /**
   * Makes it so that the client waits until the processing of the transaction whose hash is passed.
   *
   * @async
   * @param {Object} options - The options for the wait operation.
   * @param {boolean} options.waitTillMainShard - Whether to wait until the transaction is processed on the main shard.
   * @param {number} options.interval - The interval to check the transaction status.
   * @returns {Promise<ProcessedReceipt[]>} A promise that resolves to an array of processed receipts.
   * @example
   * await transaction.wait();
   */
  async wait(options?: TransactionOptions): Promise<ProcessedReceipt[]> {
    return waitTillCompleted(this.client, this.hash, options);
  }
}
