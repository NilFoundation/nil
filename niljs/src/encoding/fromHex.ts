import { BaseError } from "../errors/BaseError.js";
import type { Hex } from "../index.js";
import { addHexPrefix, removeHexPrefix } from "../utils/hex.js";

/**
 * Converts a character code to a base16 number.
 * @param charCode The character code to convert.
 * @returns The base16 representation of the input.
 */
const charCodeToBase16 = (charCode: number): number | undefined => {
  if (charCode >= 48 && charCode <= 57) {
    return charCode - 48;
  }
  if (charCode >= 65 && charCode <= 70) {
    return charCode - 55;
  }
  if (charCode >= 97 && charCode <= 102) {
    return charCode - 87;
  }
  return undefined;
};

/**
 * Converts a hex string to a number.
 * @param hex The hex string to convert.
 * @returns The number representation of the input.
 */
const hexToNumber = (hex: Hex): number => {
  return Number.parseInt(removeHexPrefix(hex), 16);
};

/**
 * Converts a hex string to a bigint.
 * @param hex The hex string to convert.
 * @returns The bigint representation of the input.
 */
const hexToBigInt = (hex: Hex): bigint => {
  return BigInt(addHexPrefix(hex));
};

/**
 * Converts a hex string to a byte array.
 *
 * @param {Hex} hex The hex string to convert.
 * @returns {Uint8Array} The byte array representation of the input.
 */
const hexToBytes = (hex: Hex): Uint8Array => {
  let hexString = hex.slice(2);
  if (hexString.length % 2) {
    hexString = `0${hexString}`;
  }

  const length = hexString.length / 2;
  const bytes = new Uint8Array(length);

  for (let index = 0, j = 0; index < length; index++) {
    const nibbleLeft = charCodeToBase16(hexString.charCodeAt(j++));
    const nibbleRight = charCodeToBase16(hexString.charCodeAt(j++));
    if (nibbleLeft === undefined || nibbleRight === undefined) {
      throw new BaseError(
        `Invalid byte sequence ("${hexString[j - 2]}${hexString[j - 1]}" in "${hexString}").`,
      );
    }
    bytes[index] = nibbleLeft * 16 + nibbleRight;
  }
  return bytes;
};

export { hexToNumber, hexToBigInt, hexToBytes };
