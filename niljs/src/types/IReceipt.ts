import type { Transaction } from "../utils/transaction.js";
import type { Hex } from "./Hex.js";
import type { ILog } from "./ILog.js";
import type { Flags } from "./RPCTransaction.js";

export type ReceiptHash = Hex | Transaction;
export type TransactionOptions = { waitTillMainShard?: boolean; interval?: number };

/**
 * The receipt interface.
 */
type Receipt = {
  flags: Flags[];
  success: boolean;
  status: string;
  failedPc: number;
  gasUsed: string;
  gasPrice?: string;
  logs: ILog[];
  transactionHash: Hex;
  contractAddress: string;
  blockHash: string;
  blockNumber: number;
  txnIndex: number;
  outTransactions: Hex[] | null;
  outputReceipts: (Receipt | null)[] | null;
  shardId: number;
  includedInMain: boolean;
  errorMessage?: string;
};

type ProcessedReceipt = Omit<Receipt, "gasUsed" | "gasPrice" | "outputReceipts"> & {
  gasUsed: bigint;
  gasPrice?: bigint;
  outputReceipts: (ProcessedReceipt | null)[] | null;
};

export type { Receipt, ProcessedReceipt };

export function CheckReceiptSuccess(receipt: ProcessedReceipt): boolean {
  if (!receipt.success) {
    return false;
  }
  if (receipt.logs.length > 0) {
    if (
      receipt.logs[0].topics.length > 0 &&
      receipt.logs[0].topics[0] ===
        "0x809af745217c49a151bbf9c1a1fcf6355da9b1f17e696139e449fdf0ba9d9423"
    ) {
      return false;
    }
  }
  return true;
}
