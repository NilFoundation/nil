import { BaseCommand } from "../../base.js";
import { hexArg } from "../../types.js";

export default class ContractTokens extends BaseCommand {
  static override summary = "Get the tokens held by a smart contract as a map tokenId -> balance";

  static flags = {};

  static args = {
    address: hexArg({
      name: "address",
      required: true,
      description: "The address of the smart contract",
    }),
  };

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  public async run(): Promise<Record<string, string>> {
    const { args } = await this.parse(ContractTokens);
    if (!this.rpcClient) {
      throw new Error("RPC client is not initialized");
    }
    const tokens = await this.rpcClient.getTokens(args.address, "latest");

    const tokensForOutput: Record<string, string> = {};
    for (const [key, value] of Object.entries(tokens)) {
      tokensForOutput[key] = value.toString();
    }

    const output = Object.entries(tokensForOutput)
      .map(([token, balance]) => `${token}: ${balance}`)
      .join("\n");

    this.log(output);

    return tokensForOutput;
  }
}
