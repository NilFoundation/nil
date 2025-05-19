import type { Hex } from "@nilfoundation/niljs";
import { Args } from "@oclif/core";
import { BaseCommand } from "../../base.js";

export default class MinterBurnTokenCommand extends BaseCommand {
  static override description = "Burn a custom token";

  static override examples = ["<%= config.bin %> <%= command.id %> 1000"];

  static args = {
    amount: Args.string({
      name: "amount",
      required: true,
      description: "Amount to burn",
    }),
  };

  async run(): Promise<Hex> {
    const { args } = await this.parse(MinterBurnTokenCommand);
    const { smartAccount } = await this.setupSmartAccount();

    if (!this.rpcClient) {
      throw new Error("RPC client is not initialized");
    }

    const amount = BigInt(args.amount);

    const tx = await smartAccount.burnToken(amount);
    await this.waitOnTx(tx.hash);

    this.info(`Burned ${amount} amount of token, TX Hash:`);
    this.log(tx.hash);

    return tx.hash;
  }
}
