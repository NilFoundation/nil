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
