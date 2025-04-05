import { BaseCommand } from "../../base.js";

export default class ConfigInit extends BaseCommand {
  static override description = "Initialize the config file";

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  public async run(): Promise<string> {
    await this.init();

    return "config initialized";
  }
}
