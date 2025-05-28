import type { ProcessedReceipt } from "@nilfoundation/niljs";
import { BaseCommand, bigIntReplacer } from "../base.js";
import { hexArg } from "../types.js";

export default class ReceiptCommand extends BaseCommand {
  static override description = "Retrieve a receipt from the cluster";

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  static args = {
    hash: hexArg({
      name: "hash",
      required: true,
      description: "transaction hash",
    }),
  };

  async run(): Promise<ProcessedReceipt | null> {
    const { args, flags } = await this.parse(ReceiptCommand);

    if (!this.rpcClient) {
      throw new Error("RPC client is not initialized");
    }

    const res = await this.rpcClient.getTransactionReceiptByHash(args.hash);
    const receiptJson = JSON.stringify(res, bigIntReplacer);
    if (flags.quiet) {
      this.log(receiptJson);
    } else {
      this.log("Receipt data:", receiptJson);
    }
    return res;
  }
}
