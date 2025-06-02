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
 sync function waitTillCompleted(
  client: PublicClient,
  hash: ReceiptHash,
  options?: TransactionOptions,
 : Promise<ProcessedReceipt[]> {
  console.log('waitTillCompleted: called with hash:', hash);

  let hashes: [Hex][];
  if (typeof hash === "string") {
    hashes = [[hash]];
    console.log('waitTillCompleted: hash is string:', hash);
  } else {
    hashes = [[hash.hash]];
    console.log('waitTillCompleted: hash is object:', hash.hash);
  }

  const interval = options?.interval || 1000;
  const waitTillMainShard = options?.waitTillMainShard || false;
  const receipts: ProcessedReceipt[] = [];
  let cur = 0;
  let loopCount = 0;
  while (cur !== hashes.length) {
    loopCount++;
    const [curHash] = hashes[cur];
    console.log(`waitTillCompleted: loop ${loopCount}, cur=${cur}, curHash=${curHash}, hashes.length=${hashes.length}`);

    let receipt;
    try {
      receipt = await client.getTransactionReceiptByHash(curHash);
      console.log('waitTillCompleted: receipt fetched:', JSON.stringify(receipt));
    } catch (err) {
      console.error('waitTillCompleted: error fetching receipt for hash:', curHash, err);
      await new Promise((resolve) => setTimeout(resolve, interval));
      continue;
    }

    if (!receipt) {
      console.log('waitTillCompleted: no receipt yet for hash:', curHash, 'Sleeping...');
      await new Promise((resolve) => setTimeout(resolve, interval));
      continue;
    }

    if (hashes.length === 1 && receipt.flags.some((x) => x === "External") && !receipt.success) {
      console.log('waitTillCompleted: single hash, external failed receipt detected:', JSON.stringify(receipt));
      return [receipt];
    }

    if (
      receipt.outTransactions !== null &&
      receipt.outputReceipts &&
      receipt.outputReceipts.filter((x) => x !== null).length !== receipt.outTransactions.length
    ) {
      console.log('waitTillCompleted: outputReceipts not ready, receipt:', JSON.stringify(receipt));
      await new Promise((resolve) => setTimeout(resolve, interval));
      continue;
    }

    if (waitTillMainShard && receipt.shardId !== 0 && !receipt.includedInMain) {
      console.log('waitTillCompleted: waiting for inclusion in main shard, receipt:', JSON.stringify(receipt));
      await new Promise((resolve) => setTimeout(resolve, interval));
      continue;
    }
    cur++;
    receipts.push(receipt);
    console.log('waitTillCompleted: receipt completed and pushed, hash:', curHash);

    if (receipt.outputReceipts) {
      for (const r of receipt.outputReceipts) {
        if (r !== null) {
          hashes.push([r.transactionHash]);
          console.log('waitTillCompleted: pushing outputReceipt hash:', r.transactionHash);
        }
      }
    }
  }

  console.log('waitTillCompleted: all receipts collected, returning:', receipts.length);
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
