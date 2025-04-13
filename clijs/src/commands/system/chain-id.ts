import { BaseCommand } from "../../base.js";

export default class ChainId extends BaseCommand {
  static override description = "Get network chain ID";

  static override examples = ["$ nil system chain-id"];

  async run(): Promise<number> {
    const { rpcClient } = this;
    if (!rpcClient) {
      this.error("RPC client is not initialized");
    }

    try {
      const chainId = await rpcClient.chainId();
      return chainId;
    } catch (error) {
      this.error(`Failed to get chain ID: ${error}`);
    }
  }
}
