import { mnemonicToSeedSync } from "@scure/bip39";
import { toHex } from "../encoding/toHex.js";
import { addHexPrefix } from "../utils/hex.js";
import type { IPrivateKey } from "./index.js";

/**
 * Generate the private key from the mnemonic phrase.
 * @param mnemonic The mnemonic phrase to generate the private key from.
 * @param password The password to generate the private key.
 * @returns The private key.
 */
const privateKeyFromPhrase = (mnemonic: string): IPrivateKey => {
  return addHexPrefix(toHex(mnemonicToSeedSync(mnemonic).slice(0, 32)));
};

export { privateKeyFromPhrase };
