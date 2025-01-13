import type { Hex } from "@nilfoundation/niljs";
import { BaseCommand } from "../../base.js";
import { ConfigKeys } from "../../common/config.js";
import { getPublicKey } from "@nilfoundation/niljs";

export default class WalletNew extends BaseCommand {
  static override description =
    "Get the address and the public key of the wallet set in the config file";

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  public async run(): Promise<{ PublicKey: Hex; Address: Hex }> {
    const privateKey = this.cfg?.[ConfigKeys.PrivateKey] as Hex;
    if (!privateKey) {
      throw new Error(
        "Private key not found in config. Perhaps you need to run 'keygen new' first?",
      );
    }

    const publicKey = getPublicKey(privateKey, true);

    const address = this.cfg?.[ConfigKeys.Address] as Hex;
    if (!address) {
      throw new Error("Address not found in config. Perhaps you need to run 'wallet new' first?");
    }

    const ret = { PublicKey: publicKey, Address: address };

    if (this.quiet) {
      this.log(address);
      this.log(publicKey);
    } else {
      this.log("Wallet address: ", address);
      this.log("Public Key: ", publicKey);
    }
    return ret;
  }
}
