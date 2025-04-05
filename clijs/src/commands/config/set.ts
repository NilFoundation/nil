import { Args } from "@oclif/core";
import { BaseCommand } from "../../base.js";
import { ConfigKeys } from "../../common/config.js";

// Supported options in the config file
const supportedOptions = new Set([
  ConfigKeys.RpcEndpoint,
  ConfigKeys.CometaEndpoint,
  ConfigKeys.FaucetEndpoint,
  ConfigKeys.PrivateKey,
  ConfigKeys.Address,
]);

export default class ConfigSet extends BaseCommand {
  static override description = "Set the value of a key in the config file";

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  static args = {
    key: Args.string({
      description: "The key to set in the config file",
      required: true,
    }),
    value: Args.string({
      description: "The value to set for the key",
      required: true,
    }),
  };

  public async run(): Promise<string> {
    const { args } = await this.parse(ConfigSet);
    const { key, value } = args;

    if (!this.configManager) {
      this.error("Config manager is not initialized");
    }

    if (!supportedOptions.has(key as ConfigKeys)) {
      this.error(`Key "${key}" is not known`);
    }

    this.configManager.updateConfig(ConfigKeys.NilSection, key, value);

    const formattedOutput = `Set "${key}" to "${value}"`;

    return formattedOutput;
  }
}
