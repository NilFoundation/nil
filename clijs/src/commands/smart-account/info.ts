import type { Hex } from "@nilfoundation/niljs";
import { getPublicKey } from "@nilfoundation/niljs";
import { BaseCommand } from "../../base.js";
import { ConfigKeys } from "../../common/config.js";

export default class SmartAccountInfo extends BaseCommand {
  static override description =
    "Get the address and the public key of the smart account set in the config file";

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  public async run(): Promise<{ PublicKey: Hex; Address: Hex }> {
    const privateKey = this.configManager?.getConfigValue(
      ConfigKeys.NilSection,
      ConfigKeys.PrivateKey,
    ) as Hex;
    if (!privateKey) {
      throw new Error(
        "Private key not found in config. Perhaps you need to run 'keygen new' first?",
      );
    }

    const publicKey = getPublicKey(privateKey, true);

    const address = this.configManager?.getConfigValue(
      ConfigKeys.NilSection,
      ConfigKeys.Address,
    ) as Hex;
    if (!address) {
      throw new Error(
        "Address not found in config. Perhaps you need to run 'smart-account new' first?",
      );
    }

    const ret = { PublicKey: publicKey, Address: address };

    if (this.quiet) {
      this.log(address);
      this.log(publicKey);
    } else {
      this.log("Smart account address: ", address);
      this.log("Public Key: ", publicKey);
    }
    return ret;
  }
}
