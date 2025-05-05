import { Args } from "@oclif/core";
import { BaseCommand } from "../../base";
import { ConfigKeys } from "../../common/config";

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

  public async run(): Promise<string> {
    const { args } = await this.parse(ConfigGet);

    const value = this.configManager!.getConfigValue(ConfigKeys.NilSection, args.name);
    if (!value) {
      this.log(`Key ${args.name} not found`);
      return "";
    }
    this.log(value);
    return value;
  }
}
