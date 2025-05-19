import { Command } from "@oclif/core";

export default class Contract extends Command {
  static override description = "Interact with the smart contracts";

  async run(): Promise<void> {
    await this.config.runCommand("help", ["contract"]);
  }
}
