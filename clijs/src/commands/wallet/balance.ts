import type { Hex } from "@nilfoundation/niljs";
import { BaseCommand } from "../../base.js";
import { ConfigKeys } from "../../common/config.js";

export default class WalletBalance extends BaseCommand {
  static override description = "Get the balance of the wallet";

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  async run(): Promise<bigint> {
    const address = this.cfg?.[ConfigKeys.Address];
    if (!address) {
      throw new Error("Invalid or missing wallet address in config.");
    }

    const rpcClient = this.rpcClient;
    if (!rpcClient) {
      throw new Error("RPC client not found.");
    }

    const balance = await rpcClient.getBalance(address as Hex, "latest");

    if (this.quiet) {
      this.log(balance.toString());
    } else {
      this.log(`Balance: ${balance.toString()}`);
    }
    return balance;
  }
}
