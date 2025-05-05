import { BaseCommand } from "../../base";

export default class ConfigShow extends BaseCommand {
  static override description = "Show the contents of the config file";

  static override examples = ["$ nil config show"];

  public async run(): Promise<string> {
    return this.configManager!.showConfig();
  }
}
