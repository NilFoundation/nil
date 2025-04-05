import { BaseCommand } from "../../base.js";
import { ConfigKeys } from "../../common/config.js";

export default class ConfigShow extends BaseCommand {
  static override description = "Show the contents of the config file";

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  public async run(): Promise<string> {
    if (!this.configManager) {
      this.error("Config manager is not initialized");
    }

    const config = this.configManager.loadConfig();
    const nilSection = config[ConfigKeys.NilSection] as Record<string, string>;

    // Build the formatted output
    let formattedOutput = "";

    if (nilSection) {
      for (const [key, value] of Object.entries(nilSection)) {
        formattedOutput += `${key.padEnd(18)}: ${value}\n`;
      }
    }

    return formattedOutput;
  }
}
