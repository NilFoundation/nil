import { generateRandomPrivateKey } from "@nilfoundation/niljs";
import { BaseCommand } from "../../base.js";
import { ConfigKeys } from "../../common/config.js";

export default class KeygenNew extends BaseCommand {
  static override description = "Generate a new key";

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  public async run(): Promise<string> {
    const privateKey = generateRandomPrivateKey().slice(2);
    this.configManager?.updateConfig(ConfigKeys.NilSection, ConfigKeys.PrivateKey, privateKey);
    if (this.quiet) {
      this.log(privateKey);
    } else {
      this.log(`Private key: ${privateKey}`);
    }
    return privateKey;
  }
}
