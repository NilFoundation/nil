import { Args } from "@oclif/core";
import { BaseCommand } from "../../base.js";
import { bigintArg, hexArg } from "../../types.js";

export default class ContractTopup extends BaseCommand {
  static override summary = "Top-up address with token";
  static override description = "Top up the smart contract specified in the config file";

  static flags = {};

  static args = {
    address: hexArg({
      name: "address",
      required: true,
      description: "The address of the smart contract",
    }),
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
    const { args, flags } = await this.parse(ContractTopup);

    if (!this.faucetClient) {
      throw new Error("Faucet client is not initialized");
    }
    if (!this.rpcClient) {
      throw new Error("RPC client is not initialized");
    }

    const faucets = await this.faucetClient.getAllFaucets();
    const faucetAddress = faucets[args.tokenId ?? "NIL"];
    const txHash = await this.faucetClient.topUpAndWaitUntilCompletion(
      {
        amount: args.amount,
        smartAccountAddress: args.address,
        faucetAddress: faucetAddress,
      },
      this.rpcClient,
    );
    this.info(`Top-up tx - ${txHash}`);
    const balance = await this.rpcClient.getBalance(args.address);
    const balances = await this.rpcClient.getTokens(args.address, "latest");
    if (!flags.quiet) {
      if (args.tokenId === "NIL") {
        this.log(`Balance: ${balance.toString()}`);
      } else {
        // biome-ignore lint/style/noNonNullAssertion:
        this.log(`Token balance: ${balances[args.tokenId!]}`);
      }
    } else {
      if (args.tokenId === "NIL") {
        this.log(balance.toString());
      } else {
        // biome-ignore lint/style/noNonNullAssertion:
        this.log(balances[args.tokenId!].toString());
      }
    }
    return true;
  }
}
