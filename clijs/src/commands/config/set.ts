import { Args } from "@oclif/core";
import { BaseCommand } from "../../base";
import { ConfigKeys } from "../../common/config";

export default class ConfigSet extends BaseCommand {
  static override description = "Set the value of a key in the config file";

  static override examples = ["$ nil config set rpc_endpoint http://localhost:1234"];

  static args = {
    key: Args.string({
      name: "key",
      required: true,
    }),
    value: Args.string({
      name: "value",
      required: true,
    }),
  };

  public async run(): Promise<undefined> {
    const { args } = await this.parse(ConfigSet);

    this.configManager!.updateConfig(ConfigKeys.NilSection, args.key, args.value);
    this.log(`Set ${args.key} to ${args.value}`);
  }
}
