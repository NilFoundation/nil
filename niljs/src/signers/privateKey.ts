import { secp256k1 } from "@noble/curves/secp256k1";
import { toHex } from "../encoding/toHex.js";
import { getPublicKey } from "./publicKey.js";
import type { IPrivateKey } from "./types/IPrivateKey.js";

/**
 * Generate a new private key.
 * @returns The new private key
 * @example
 * const privateKey = generatePrivateKey();
 */
const generateRandomPrivateKey = (): IPrivateKey => toHex(secp256k1.utils.randomPrivateKey());

/**
 * Generate a new private key and its corresponding public key.
 * @returns The new private key and its corresponding public key
 * @example
 * const { privateKey, publicKey } = generateKeyPair();
 * console.log(privateKey);
 * console.log(publicKey);
 * // => "0x..."
 * // => "0x..."
 */
const generateRandomKeyPair = () => {
  const privateKey = generateRandomPrivateKey();
  const publicKey = getPublicKey(privateKey);
  return { privateKey, publicKey };
};

export { generateRandomPrivateKey, generateRandomKeyPair };
