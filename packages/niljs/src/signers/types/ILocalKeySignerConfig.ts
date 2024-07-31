import type { IPrivateKey } from "./IPrivateKey.js";

/**
 * The interface for the configuration object of LocalKeySigner.
 */
type ILocalKeySignerConfig = {
  /**
   * The private key to sign the data.
   * @example '4b3b4c4d4e4f505152535455565758595a6162636465666768696a6b6c6d6e6f'
   */
  privateKey: IPrivateKey;
  /**
   * The mnemonic to derive the private key.
   * If the mnemonic is provided, privateKey will be ignored.
   */
  mnemonic?: string;
};

export type { ILocalKeySignerConfig };
