import type { ProcessedReceipt } from "@nilfoundation/niljs";
import { BaseCommand } from "../base.js";
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
    if (flags.quiet) {
      this.log(res as unknown as string);
    } else {
      this.log("Receipt data:", res as unknown as string);
    }
    return res;
  }
}
