import { Args } from "@oclif/core";
import { BaseCommand } from "../../base.js";

export default class GasPrice extends BaseCommand {
    static override description = "Get network gas price";

    static override examples = ["$ nil system gas-price <shard-id>"];

    static args = {
        shardId: Args.integer({
            description: "Shard number",
            required: true,
        }),
    };

    async run(): Promise<bigint> {
        const { rpcClient } = this;
        if (!rpcClient) {
            this.error("RPC client is not initialized");
        }

        const { args } = await this.parse(GasPrice);
        const { shardId } = args;

        try {
            const gasPrice = await rpcClient.getGasPrice(shardId);
            return gasPrice;
        } catch (error) {
            this.error(`Failed to get gas price: ${error}`);
        }
    }
} 