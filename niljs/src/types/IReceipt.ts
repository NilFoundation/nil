import type { Hex } from "./Hex.js";
import type { ILog } from "./ILog.js";

/**
 * The receipt interface.
 */
type IReceipt = {
  success: boolean;
  gasUsed: string;
  gasPrice?: string;
  bloom: string;
  logs: ILog[];
  messageHash: Hex;
  contractAddress: string;
  blockHash: string;
  blockNumber: number;
  msgIndex: number;
  outMessages: Hex[] | null;
  outputReceipts: (IReceipt | null)[] | null;
  shardId: number;
  includedInMain: boolean;
};

type ProcessedReceipt = Omit<IReceipt, "gasUsed" | "gasPrice" | "outputReceipts"> & {
  gasUsed: bigint;
  gasPrice?: bigint;
  outputReceipts: (ProcessedReceipt | null)[] | null;
};

export type { IReceipt, ProcessedReceipt };
