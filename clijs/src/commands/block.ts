import { type Block, type BlockTag, type Hex, toHex } from "@nilfoundation/niljs";
import { Args, Flags } from "@oclif/core";
import { BaseCommand } from "../base.js";

export default class BlockCommand extends BaseCommand {
  static override description = "Retrieve a block from the cluster";

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  static flags = {
    shardId: Flags.integer({
      char: "s",
      description: "Block shard ID",
      required: false,
    }),
  };

  static args = {
    blockId: Args.string({
      name: "blockId",
      required: true,
      description: "Number, hash or tag",
      default: "latest",
    }),
  };

  async run(): Promise<Block<boolean>> {
    const { args, flags } = await this.parse(BlockCommand);

    if (!this.rpcClient) {
      throw new Error("RPC client is not initialized");
    }

    if (/^0x[0-9a-fA-F]+$/.test(args.blockId)) {
      return await this.rpcClient.getBlockByHash(args.blockId as Hex);
      // biome-ignore lint/style/noUselessElse: <explanation>
    } else if (validBlockTags.includes(args.blockId as BlockTag)) {
      return await this.rpcClient.getBlockByNumber(
        args.blockId as BlockTag,
        undefined,
        flags.shardId,
      );
    }
    return await this.rpcClient.getBlockByNumber(toHex(args.blockId), undefined, flags.shardId);
  }
}

const validBlockTags: BlockTag[] = ["latest", "earliest", "pending"];
