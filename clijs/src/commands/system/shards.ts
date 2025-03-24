import { BaseCommand } from "../../base.js";

export default class Shards extends BaseCommand {
    static override description = "Print the list of shards";

    static override examples = ["$ nil system shards"];

    async run(): Promise<void> {
        const { rpcClient } = this;
        if (!rpcClient) {
            this.error("RPC client is not initialized");
        }

        try {
            const shards = await rpcClient.getShardIdList();
            this.info(JSON.stringify(shards, null, 2));
        } catch (error) {
            this.error(`Failed to get shards: ${error}`);
        }
    }
} 