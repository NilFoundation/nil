import { IntegerOutOfRangeError } from "../errors/encoding.js";
import type { Hex } from "../types/Hex.js";
import { addHexPrefix, isHexString } from "../utils/hex.js";

const hexes = Array.from({ length: 256 }, (Char, i) => i.toString(16).padStart(2, "0"));

/**
 * Converts a string to a hex string.
 * @param str The input string to convert.
 * @returns The hex string representation of the input.
 */
const stringToHex = (str: string): Hex => {
  let hex = "";

  for (let i = 0; i < str.length; i++) {
    hex += str.charCodeAt(i).toString(16);
  }

  return addHexPrefix(hex);
};

/**
 * Converts bytes to a hex string.
 * @param bytes The bytes to convert.
 * @returns The hex string representation of the input.
 */
const bytesToHex = (bytes: Uint8Array): Hex => {
  let hex = "";

  for (let i = 0; i < bytes.length; i++) {
    hex += hexes[bytes[i]];
  }

  return addHexPrefix(hex);
};

/**
 * Converts an unsigned number to a hex string.
 * @param num The number to convert.
 * @returns The hex string representation of the input.
 */
const numberToHex = (num: number | bigint): Hex => {
  const value = BigInt(num);
  const maxValue = BigInt(Number.MAX_SAFE_INTEGER);
  const minValue = 0;

  if (typeof num !== "bigint" && ((maxValue && value > maxValue) || value < minValue)) {
    throw new IntegerOutOfRangeError({
      max: maxValue,
      min: minValue,
      value,
    });
  }

  return addHexPrefix(value.toString(16));
};

/**
 * Converts a string, number, bigint, boolean, or ByteArrayType to a hex string.
 * @param value The input to convert.
 * @returns The hex string representation of the input.
 */
const toHex = <T extends string | Uint8Array | boolean | bigint | number>(value: T): Hex => {
  if (typeof value === "string") {
    const isHex = isHexString(value);

    return isHex ? value : stringToHex(value);
  }

  if (value instanceof Uint8Array) {
    return bytesToHex(value);
  }

  if (typeof value === "number" || typeof value === "bigint") {
    return numberToHex(value);
  }

  return addHexPrefix((value ? 1 : 0).toString(16));
};

export { toHex };
