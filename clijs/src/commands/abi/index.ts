import { Command } from "@oclif/core";

export default class AbiCommand extends Command {
  static override description = "Encode or decode a contract call using the provided ABI";

  async run(): Promise<void> {
    await this.config.runCommand("help", ["abi"]);
  }
}
