import { BaseCommand } from "../../base.js";
import { hexArg } from "../../types.js";

export default class ContractSeqno extends BaseCommand {
  static override summary = "Get the seqno";
  static override description = "Get the seqno of the smart contract";

  static flags = {};

  static args = {
    address: hexArg({
      name: "address",
      required: true,
      description: "The address of the smart contract",
    }),
  };

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  public async run(): Promise<number> {
    const { args } = await this.parse(ContractSeqno);
    if (!this.rpcClient) {
      throw new Error("RPC client is not initialized");
    }
    const seqno = await this.rpcClient.getTransactionCount(args.address);

    this.log(seqno.toString());

    return seqno;
  }
}
