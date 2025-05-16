import { Args } from "@oclif/core";
import { BaseCommand } from "../../base.js";
import { ConfigKeys } from "../../common/config.js";

export default class ConfigGet extends BaseCommand {
  static override description = "Get the value of a key from the config file";

  static override examples = ["$ nil config get rpc_endpoint"];

  static args = {
    name: Args.string({
      name: "name",
      required: true,
      description: "The path to the bytecode file",
    }),
  };

  public async run(): Promise<string | null> {
    const { args } = await this.parse(ConfigGet);

    if (!this.configManager) {
      throw new Error("Config is not initialized");
    }
    const value = this.configManager.getConfigValue(ConfigKeys.NilSection, args.name);
    if (!value) {
      return null;
    }
    this.log(value);
    return value;
  }
}
