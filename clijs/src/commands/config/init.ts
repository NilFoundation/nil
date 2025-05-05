import { BaseCommand } from "../../base.js";

export default class ConfigInit extends BaseCommand {
  static override description = "Initialize the config file";

  static override examples = ["$ nil config init"];

  public async run(): Promise<undefined> {
    if (!this.configManager) {
      this.log("Config manager not initialized");
      return;
    }
  }
}
