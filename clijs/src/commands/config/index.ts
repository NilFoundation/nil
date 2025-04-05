import { Command } from "@oclif/core";

export default class Config extends Command {
  static override description = "Manage the =nil; CLI config";

  async run(): Promise<void> {
    await this.config.runCommand("help", ["config"]);
  }
}
