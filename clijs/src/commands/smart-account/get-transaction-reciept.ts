import type { Hex } from "@nilfoundation/niljs";
import { Args } from "@oclif/core";
import { BaseCommand } from "../../base.js";

export default class SmartAccountGetTransactionReceipt extends BaseCommand {
  static override summary = "Fetch a transaction receipt using its hash";
  static override description =
    "Retrieve the details of a transaction receipt from the blockchain using the transaction hash.";

  static args = {
    hash: Args.string({
      name: "hash",
      required: true,
      description: "The transaction hash",
    }),
  };

  static override examples = [
    "<%= config.bin %> <%= command.id %> 0x123456789abcdef",
  ];

  public async run(): Promise<void> {
    const { args } = await this.parse(SmartAccountGetTransactionReceipt);
    const { smartAccount } = await this.setupSmartAccount();

    const hash = args.hash as Hex;
    if (!hash.startsWith("0x")) {
      this.error("Invalid transaction hash. It should start with '0x'.");
    }

    this.info(`Fetching receipt for transaction hash: ${hash}...`);

    try {
      const receipt = await smartAccount.getTransactionReceipt(hash);
      if (!receipt) {
        this.error("Transaction receipt not found.");
      }

      this.log("Transaction Receipt:", JSON.stringify(receipt, null, 2));
    } catch (error) {
      this.error(
        `Failed to fetch transaction receipt: ${(error as Error).message}`
      );
    }
  }
}
