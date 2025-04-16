import { Args } from "@oclif/core";
import { BaseCommand } from "../../base.js";
import { bigintArg } from "../../types.js";

export default class SmartAccountTopup extends BaseCommand {
  static override summary = "Top-up address with token";
  static override description = "Top up the smart account specified in the config file";

  static flags = {};

  static args = {
    amount: bigintArg({
      name: "amount",
      required: false,
      description: "The path to the bytecode file",
      default: BigInt("1000000000000000000"),
    }),
    tokenId: Args.string({
      name: "token-id",
      required: false,
      description: "Token Id to top-up",
    }),
  };

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  public async run(): Promise<boolean> {
    const { args } = await this.parse(SmartAccountTopup);

    if (!this.faucetClient) {
      throw new Error("Faucet client is not initialized");
    }
    if (!this.rpcClient) {
      throw new Error("RPC client is not initialized");
    }

    const { smartAccount } = await this.setupSmartAccount();
    const faucets = await this.faucetClient.getAllFaucets();
    const faucetAddress = faucets[args.tokenId ?? "NIL"];
    const txHash = await this.faucetClient.topUpAndWaitUntilCompletion(
      {
        amount: args.amount,
        smartAccountAddress: smartAccount.address,
        faucetAddress: faucetAddress,
      },
      this.rpcClient,
    );
    this.info(`Top-up tx - ${txHash}`);
    return true;
  }
}
