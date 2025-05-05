import { BaseCommand } from "../../base.js";
import { ConfigKeys } from "../../common/config.js";

export default class ConfigShow extends BaseCommand {
  static override description = "Show the contents of the config file";

  static override examples = ["$ nil config show"];

  public async run(): Promise<string> {
    const config = this.configManager!.loadConfig();
    const nilSection = config[ConfigKeys.NilSection] as Record<string, string>;

    let formattedOutput = "";
    if (nilSection) {
      for (const [key, value] of Object.entries(nilSection)) {
        formattedOutput += `${key.padEnd(18)}: ${value}\n`;
      }
    }

    return formattedOutput;
  }
}
