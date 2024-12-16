import { Command } from "@oclif/core";

export default class Wallet extends Command {
  static override description = "Interact with the wallet set in the config file";

  async run(): Promise<void> {
    await this.config.runCommand("help", ["wallet"]);
  }
}
