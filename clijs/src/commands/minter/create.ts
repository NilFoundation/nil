import type { Hex } from "@nilfoundation/niljs";
import { Args } from "@oclif/core";
import { BaseCommand } from "../../base.js";

export default class MinterCreateTokenCommand extends BaseCommand {
  static override description = "Create a custom token";

  static override examples = ["<%= config.bin %> <%= command.id %> 0x123... 1000 MyToken"];

  static args = {
    amount: Args.string({
      name: "amount",
      required: true,
      description: "Initial token amount",
    }),
    name: Args.string({
      name: "name",
      required: true,
      description: "Token name",
    }),
  };

  async run(): Promise<Hex> {
    const { args } = await this.parse(MinterCreateTokenCommand);
    const { smartAccount } = await this.setupSmartAccount();

    if (!this.rpcClient) {
      throw new Error("RPC client is not initialized");
    }

    const amount = BigInt(args.amount);
    const name = args.name;

    const setNameTx = await smartAccount.setTokenName(name);
    await this.waitOnTx(setNameTx.hash);

    const mintTx = await smartAccount.mintToken(amount);
    await this.waitOnTx(mintTx.hash);

    const tokenId = smartAccount.address;

    this.info("Created Token ID:");
    this.log(tokenId);

    return tokenId;
  }
}
