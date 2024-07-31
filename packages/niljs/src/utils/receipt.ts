import type { PublicClient } from "../clients/PublicClient.js";
import type { Hex } from "../types/Hex.js";
import type { ProcessedReceipt } from "../types/IReceipt.js";

/**
 * Makes it so that the client waits until the processing of the message whose hash is passed.
 *
 * @async
 * @param {PublicClient} client The client that must wait for action completion.
 * @param {number} shardId The ID of the shard where the message is processed.
 * @param {Hex} hash The message hash.
 * @returns {unknown}
 * @example
 * await waitTillCompleted(client, 1, hash);
 */
export const waitTillCompleted = async (
  client: PublicClient,
  shardId: number,
  hash: Hex,
) => {
  const receipts: ProcessedReceipt[] = [];
  const hashes: [number, Hex][] = [[shardId, hash]];
  let cur = 0;
  while (cur !== hashes.length) {
    const [shardId, hash] = hashes[cur];
    const receipt = await client.getMessageReceiptByHash(hash, shardId);
    if (
      !receipt ||
      (receipt.outMessages !== null &&
        receipt.outputReceipts &&
        receipt.outputReceipts.filter((x) => x !== null).length !==
          receipt.outMessages.length)
    ) {
      await new Promise((resolve) => setTimeout(resolve, 1000));
      continue;
    }
    cur++;
    receipts.push(receipt);
    if (receipt.outputReceipts) {
      for (const r of receipt.outputReceipts) {
        if (r !== null) hashes.push([r.shardId, r.messageHash]);
      }
    }
  }

  return receipts;
};
