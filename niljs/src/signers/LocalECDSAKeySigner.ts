import { concatBytes, numberToBytesBE } from "@noble/curves/abstract/utils";
import { secp256k1 } from "@noble/curves/secp256k1";
import invariant from "tiny-invariant";
import { type Hex, bytesToHex, hexToBytes } from "../index.js";
import { assertIsValidPrivateKey } from "../utils/assert.js";
import { addHexPrefix, removeHexPrefix } from "../utils/hex.js";
import { privateKeyFromPhrase } from "./mnemonic.js";
import { getAddressFromPublicKey, getPublicKey } from "./publicKey.js";
import type { IAddress } from "./types/IAddress.js";
import type { ILocalKeySignerConfig } from "./types/ILocalKeySignerConfig.js";
import type { ISigner } from "./types/ISigner.js";

/**
 * The LocalKeySigner is a class that allows for signing data with the private key.
 * It uses the secp256k1 curve implementation by the @noble/curves/secp256k1 library.
 * @example
 * import {
 *   LocalECDSAKeySigner,
 *   generateRandomPrivateKey,
 * } from '@nilfoundation/niljs';
 * const signer = new LocalECDSAKeySigner({
 *   privateKey: generateRandomPrivateKey(),
 * });
 */
class LocalECDSAKeySigner implements ISigner {
  /**
   * The private key to be used for signing.
   *
   * @private
   * @type {Hex}
   */
  private privateKey: Hex;
  /**
   * The public key to be used for signing.
   *
   * @private
   * @type {?Hex}
   */
  private publicKey?: Hex = undefined;
  /**
   * The wallet address.
   *
   * @private
   * @type {?IAddress}
   */
  private address?: IAddress = undefined;

  /**
   * Creates an instance of LocalECDSAKeySigner.
   *
   * @constructor
   * @param {ILocalKeySignerConfig} config The config for the LocalECDSAKeySigner. See {@link ILocalKeySignerConfig}.
   */
  constructor(config: ILocalKeySignerConfig) {
    const { privateKey, mnemonic } = config;

    invariant(privateKey || mnemonic, "Either privateKey or mnemonic must be provided.");

    const privKey = mnemonic ? privateKeyFromPhrase(mnemonic) : addHexPrefix(privateKey as string);

    assertIsValidPrivateKey(privKey);

    this.privateKey = privKey;
  }

  /**
   * Signs the data.
   *
   * @public
   * @async
   * @param {Uint8Array} data The input data.
   * @returns {Promise<Uint8Array>} The signed data.
   */
  public async sign(data: Uint8Array) {
    const signature = secp256k1.sign(data, removeHexPrefix(this.privateKey));
    const { r, s, recovery } = signature;

    return concatBytes(
      numberToBytesBE(r, 32),
      numberToBytesBE(s, 32),
      numberToBytesBE(recovery, 1),
    );
  }

  /**
   * Retrieves the public key.
   *
   * @public
   * @returns {Uint8Array} The publc key of the signer.
   */
  public getPublicKey() {
    if (this.publicKey) {
      return hexToBytes(this.publicKey);
    }

    const publicKey = getPublicKey(this.privateKey, true);

    this.publicKey = publicKey;
    return hexToBytes(this.publicKey);
  }

  /**
   * Retrieves the wallet address.
   *
   * @public
   * @param {number} shardId The ID of the shard where the wallet is deployed.
   * @returns {Uint8Array} The wallet address.
   */
  public getAddress(shardId: number) {
    if (this.address) {
      return hexToBytes(this.address);
    }

    const pubKey = this.getPublicKey();
    this.address = getAddressFromPublicKey(bytesToHex(pubKey), shardId);

    return hexToBytes(this.address);
  }
}

export { LocalECDSAKeySigner };
