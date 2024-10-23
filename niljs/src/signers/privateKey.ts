import { secp256k1 } from "@noble/curves/secp256k1";
import { toHex } from "../encoding/toHex.js";
import type { IPrivateKey } from "./types/IPrivateKey.js";

/**
 * Generate a new private key.
 * @returns The new private key
 * @example
 * const privateKey = generatePrivateKey();
 */
const generateRandomPrivateKey = (): IPrivateKey => toHex(secp256k1.utils.randomPrivateKey());

export { generateRandomPrivateKey };
