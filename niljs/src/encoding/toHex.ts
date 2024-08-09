import { type Hex, IntegerOutOfRangeError, addHexPrefix } from "../index.js";

// biome-ignore lint/style/useNamingConvention: <explanation>
const hexes = Array.from({ length: 256 }, (_, i) =>
  i.toString(16).padStart(2, "0"),
);

/**
 * Convert a string to a hex string.
 * @param str - The input string to convert.
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
 * Convert bytes to a hex string.
 * @param bytes - The bytes to convert.
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
 * Convert an unsigned number to a hex string.
 * @param num - The number to convert.
 * @returns The hex string representation of the input.
 */
const numberToHex = (num: number | bigint): Hex => {
  const value = BigInt(num);
  const maxValue = BigInt(Number.MAX_SAFE_INTEGER);
  const minValue = 0;

  if (
    typeof num !== "bigint" &&
    ((maxValue && value > maxValue) || value < minValue)
  ) {
    throw new IntegerOutOfRangeError({
      max: maxValue,
      min: minValue,
      value,
    });
  }

  return addHexPrefix(value.toString(16));
};

/**
 * Convert a string, number, bigint, boolean, or ByteArrayType to a hex string.
 * @param value - The input to convert.
 * @returns The hex string representation of the input.
 */
const toHex = <T extends string | Uint8Array | boolean | bigint | number>(
  value: T,
): Hex => {
  if (typeof value === "string") {
    return stringToHex(value);
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
