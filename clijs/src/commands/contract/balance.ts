import type { Hex } from "@nilfoundation/niljs";
import { BaseCommand } from "../../base.js";
import { hexArg } from "../../types.js";

export default class ContractBalance extends BaseCommand {
  static override description = "Get the balance of the contract";

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  static args = {
    address: hexArg({
      name: "address",
      required: true,
      description: "Contract address",
    }),
  };

  async run(): Promise<bigint> {
    const { args } = await this.parse(ContractBalance);

    const rpcClient = this.rpcClient;
    if (!rpcClient) {
      throw new Error("RPC client not found.");
    }

    const balance = await rpcClient.getBalance(args.address as Hex, "latest");

    if (this.quiet) {
      this.log(balance.toString());
    } else {
      this.log(`Balance: ${balance.toString()}`);
    }
    return balance;
  }
}
