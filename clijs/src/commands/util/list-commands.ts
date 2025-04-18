import { BaseCommand } from "../../base.js";

export default class ListCommands extends BaseCommand {
  static hidden = true;

  static override description = "Print the list of all public commands";

  static override examples = ["$ nil util list-commands"];

  async run(): Promise<void> {
    await this.config.load();

    const allPlugins = Array.from(this.config.plugins.values());
    const allCommands = allPlugins.flatMap((p) => p.commands);

    for (const cmd of allCommands) {
      if (!cmd.id.startsWith("util")) {
        this.log(`${cmd.id}`);
      }
    }
  }
}
