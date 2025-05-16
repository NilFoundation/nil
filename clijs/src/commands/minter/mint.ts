import type { Hex } from "@nilfoundation/niljs";
import { Args } from "@oclif/core";
import { BaseCommand } from "../../base.js";

export default class MinterMintTokenCommand extends BaseCommand {
  static override description = "Mint a custom token";

  static override examples = ["<%= config.bin %> <%= command.id %> 1000"];

  static args = {
    amount: Args.string({
      name: "amount",
      required: true,
      description: "Amount to mint",
    }),
  };

  async run(): Promise<Hex> {
    const { args } = await this.parse(MinterMintTokenCommand);
    const { smartAccount } = await this.setupSmartAccount();

    if (!this.rpcClient) {
      throw new Error("RPC client is not initialized");
    }

    const amount = BigInt(args.amount);

    const tx = await smartAccount.mintToken(amount);
    await this.waitOnTx(tx.hash);

    this.info(`Minted ${amount} amount of token, TX Hash:`);
    this.log(tx.hash);

    return tx.hash;
  }
}
