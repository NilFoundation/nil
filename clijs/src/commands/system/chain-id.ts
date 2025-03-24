import { BaseCommand } from "../../base.js";

export default class ChainId extends BaseCommand {
    static override description = "Get network chain ID";

    static override examples = ["$ nil system chain-id"];

    async run(): Promise<void> {
        const { rpcClient } = this;
        if (!rpcClient) {
            this.error("RPC client is not initialized");
        }

        try {
            const chainId = await rpcClient.chainId();
            this.info(chainId.toString());
        } catch (error) {
            this.error(`Failed to get chain ID: ${error}`);
        }
    }
} 