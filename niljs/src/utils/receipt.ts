import type { PublicClient } from "../clients/PublicClient.js";
import type { Hex } from "../types/Hex.js";
import type {
  ProcessedReceipt,
  Receipt,
  ReceiptHash,
  TransactionOptions,
} from "../types/IReceipt.js";

/**
 * Makes it so that the client waits until the processing of the transaction whose hash is passed.
 *
 * @async
 * @param {PublicClient} client The client that must wait for action completion.
 * @param {Hex} hash The transaction hash.
 * @returns {unknown}
 * @example
 * await waitTillCompleted(client, hash);
 */
async function waitTillCompleted(
  client: PublicClient,
  hash: ReceiptHash,
  options?: TransactionOptions,
): Promise<ProcessedReceipt[]> {
  let hashes: [Hex][];
  if (typeof hash === "string") {
    hashes = [[hash]];
  } else {
    hashes = [[hash.hash]];
  }

  const interval = options?.interval || 1000;
  const waitTillMainShard = options?.waitTillMainShard || false;
  const receipts: ProcessedReceipt[] = [];
  let cur = 0;
  while (cur !== hashes.length) {
    const [hash] = hashes[cur];
    const receipt = await client.getTransactionReceiptByHash(hash);

    if (!receipt) {
      await new Promise((resolve) => setTimeout(resolve, interval));
      continue;
    }

    if (hashes.length === 1 && receipt.flags.some((x) => x === "External") && !receipt.success) {
      return [receipt];
    }

    if (
      receipt.outTransactions !== null &&
      receipt.outputReceipts &&
      receipt.outputReceipts.filter((x) => x !== null).length !== receipt.outTransactions.length
    ) {
      await new Promise((resolve) => setTimeout(resolve, interval));
      continue;
    }

    if (waitTillMainShard && receipt.shardId !== 0 && !receipt.includedInMain) {
      await new Promise((resolve) => setTimeout(resolve, interval));
      continue;
    }
    cur++;
    receipts.push(receipt);
    if (receipt.outputReceipts) {
      for (const r of receipt.outputReceipts) {
        if (r !== null) hashes.push([r.transactionHash]);
      }
    }
  }

  return receipts;
}

function mapReceipt(receipt: Receipt): ProcessedReceipt {
  return {
    ...receipt,
    gasUsed: BigInt(receipt.gasUsed),
    gasPrice: receipt.gasPrice ? BigInt(receipt.gasPrice) : 0n,
    outputReceipts:
      receipt.outputReceipts?.map((x) => {
        if (x === null) {
          return null;
        }
        return mapReceipt(x);
      }) ?? null,
  };
}

export { waitTillCompleted, mapReceipt };
