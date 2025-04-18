import { Args } from "@oclif/core";
import { BaseCommand } from "../../base.js";
import { ConfigKeys } from "../../common/config.js";

export default class ConfigGet extends BaseCommand {
  static override description = "Get the value of a key from the config file";

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  static args = {
    key: Args.string({
      description: "The key to get from the config file",
      required: true,
    }),
  };

  public async run(): Promise<string | undefined> {
    const { args } = await this.parse(ConfigGet);
    const { key } = args;

    if (!this.configManager) {
      this.error("Config manager is not initialized");
    }

    const value = this.configManager.getConfigValue(ConfigKeys.NilSection, key);

    if (value === undefined) {
      this.warn(`Key "${key}" is not found in the config file`);
      return undefined;
    }

    if (this.quiet) {
      this.log(value);
    } else {
      this.log(`${key}: ${value}`);
    }

    return value;
  }
}
