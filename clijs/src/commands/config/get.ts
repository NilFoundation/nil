import {BaseCommand} from "../../base";
import {ConfigKeys} from "../../common/config";
import {Args} from "@oclif/core";

export default class ConfigGet extends BaseCommand {
  static override description = "Get the value of a key from the config file";

  static override examples = ["$ nil config get rpc_endpoint"];

  static args = {
    filename: Args.string({
      name: "name",
      required: true,
      description: "The path to the bytecode file",
    }),
  };

  public async run(): Promise<string> {
    const privateKey = generateRandomPrivateKey().slice(2);

    this.configManager.getConfigValue(
      ConfigKeys.NilSection,
      ConfigKeys.FaucetEndpoint,
      rpcEndpoint,
    )


    this.configManager?.updateConfig(ConfigKeys.NilSection, ConfigKeys.PrivateKey, privateKey);
    if (this.quiet) {
      this.log(privateKey);
    } else {
      this.log(`Private key: ${privateKey}`);
    }
    return privateKey;
  }
}
