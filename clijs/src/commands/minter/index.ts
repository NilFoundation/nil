import { BaseCommand } from "../../base.js";

export default class MinterCommand extends BaseCommand {
  static override description = "Interact with the minter on the cluster";

  async run(): Promise<void> {
    await this.config.runCommand("help", ["minter"]);
  }
}
