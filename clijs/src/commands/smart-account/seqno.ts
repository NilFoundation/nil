import { BaseCommand } from "../../base.js";

export default class SmartAccountSeqno extends BaseCommand {
  static override summary = "Get the seqno";
  static override description =
    "Get the seqno of the smart account whose address specified in config.address field";

  static flags = {};

  static args = {};

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  public async run(): Promise<number> {
    const { smartAccount } = await this.setupSmartAccount();
    return await smartAccount.client.getTransactionCount(smartAccount.address);
  }
}
