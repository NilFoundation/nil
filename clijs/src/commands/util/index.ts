import { Command } from "@oclif/core";

export default class UtilCommand extends Command {
  static hidden = true;

  static override description = "Utility commands (e.g. for testing)";

  async run(): Promise<void> {
    await this.config.runCommand("help", ["util"]);
  }
}
