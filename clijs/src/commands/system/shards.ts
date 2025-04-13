import { BaseCommand } from "../../base.js";

export default class Shards extends BaseCommand {
  static override description = "Print the list of shards";

  static override examples = ["$ nil system shards"];

  async run(): Promise<number[]> {
    const { rpcClient } = this;
    if (!rpcClient) {
      this.error("RPC client is not initialized");
    }

    try {
      const shards = await rpcClient.getShardIdList();
      return shards;
    } catch (error) {
      this.error(`Failed to get shards: ${error}`);
    }
  }
}
