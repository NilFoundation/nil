import type { Hex } from "./Hex.js";
import type { ProcessedMessage } from "./ProcessedMessage.js";

/**
 * The block type.
 * Type `T` determines whether the block contains processed messages or just message hashes.
 */
type Block<T = false> = {
  number: number;
  hash: Hex;
  parentHash: Hex;
  inMessagesRoot: Hex;
  receiptsRoot: Hex;
  shardId: number;
  messages: T extends true ? Array<ProcessedMessage> : Array<Hex>;
};

/**
 * The block tag type.
 */
type BlockTag = "latest" | "earliest" | "pending";

export type { Block, BlockTag };
