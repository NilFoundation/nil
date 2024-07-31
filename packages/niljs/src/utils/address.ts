import { numberToBytesBE } from "@noble/curves/abstract/utils";
import { poseidonHash } from "../encoding/poseidon.js";
import { hexToBytes } from "../index.js";
import type { IAddress } from "../signers/types/IAddress.js";

/**
 * The regular expression for matching addresses.
 *
 */
const ADDRESS_REGEX = /^0x[0-9a-fA-F]{40}$/;

/**
 * Checks if the value is an address. If the value is an address, returns true.
 * Otherwise, returns false.
 * @param value The value to check.
 */
const isAddress = (value: string): value is IAddress => {
  return typeof value === "string" && ADDRESS_REGEX.test(value);
};

/**
 * Returns the ID of the shard containing the provided address.
 * @param address The address.
 */
const getShardIdFromAddress = (address: string): number => {
  if (typeof address === "string") {
    return Number.parseInt(address.slice(2, 6), 16);
  }

  return (address[0] << 8) | address[1];
};

/**
 * Calculates an address.
 *
 * @param {number} shardId The ID of the shard containing the address.
 * @param {Uint8Array} code The bytecode to be deployed at the address.
 * @param {Uint8Array} salt Arbitrary data for address generation.
 * @returns {Uint8Array} The address.
 */
const calculateAddress = (
  shardId: number,
  code: Uint8Array,
  salt: Uint8Array,
): Uint8Array => {
  if (!Number.isInteger(shardId)) {
    throw new Error("Shard ID must be an integer");
  }
  if (salt.length !== 32) {
    throw new Error("Salt must be 32 bytes");
  }
  if (code.length === 0) {
    throw new Error("Code must not be empty");
  }
  if (shardId < 0 || shardId > 0xffff) {
    throw new Error("Invalid shard ID");
  }
  const bytes = new Uint8Array(code.length + 32);
  bytes.set(code);
  bytes.set(salt, code.length);
  const hash = poseidonHash(bytes);
  const shardPart = numberToBytesBE(shardId, 2);
  const hashPart = numberToBytesBE(hash, 32);

  return new Uint8Array([...shardPart, ...hashPart.slice(14)]);
};

/**
 * Refines the address.
 *
 * @param {(Uint8Array | `0x`)} address The address to refine.
 * @returns {Uint8Array} The refined address.
 */
const refineAddress = (address: Uint8Array | `0x${string}`): Uint8Array => {
  if (typeof address === "string") {
    const bytes = hexToBytes(address);
    if (bytes.length !== 20) {
      throw new Error("Invalid address length");
    }
    return bytes;
  }
  if (address.length !== 20) {
    throw new Error("Invalid address length");
  }
  return address;
};

export { isAddress, getShardIdFromAddress, calculateAddress, refineAddress };
