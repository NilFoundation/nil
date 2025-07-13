import { keccak_256 } from "@noble/hashes/sha3";
import { bytesToHex } from "../encoding/fromBytes.js";
import type { IAddress } from "../signers/types/IAddress.js";
import type { Hex } from "../types/Hex.js";
import { removeHexPrefix } from "./hex.js";

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
const getShardIdFromAddress = (address: Hex): number => {
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
const calculateAddress = (shardId: number, code: Uint8Array, salt: Uint8Array): Uint8Array => {
  if (salt.length !== 32) {
    throw new Error("Salt must be 32 bytes");
  }
  if (code.length === 0) {
    throw new Error("Code must not be empty");
  }

  return calculateAddress2(shardId, code, salt);
};

const calculateAddress2 = (shardId: number, code: Uint8Array, salt: Uint8Array): Uint8Array => {
  // 1 byte is 0xFF, 20 bytes for sender, 32 bytes for salt, 32 bytes for code hash
  const data = new Uint8Array(1 + 20 + 2 * 32);
  data.set([0xff], 0); // 1 byte for shard ID
  data[1] = (shardId >> 8) & 0xff;
  data[2] = shardId & 0xff;
  data.fill(0x33, 3, 3 + 18); // Relayer address
  data.set(salt, 21); // 32 bytes for salt
  data.set(keccak_256(code), 53); // 32 bytes for code hash

  const addr = new Uint8Array(20);
  addr[0] = (shardId >> 8) & 0xff; // 2 bytes for shard ID
  addr[1] = shardId & 0xff;
  const addrData = keccak_256(data);
  addr.set(addrData.slice(32 - 20 + 2), 2); // 18 bytes for address
  return addr;
};

/**
 * Refines the address.
 *
 * @param {(Uint8Array | Hex)} address The address to refine.
 * @returns {Hex} The refined address.
 */
const refineAddress = (address: Uint8Array | Hex): Hex => {
  if (typeof address === "string") {
    if (removeHexPrefix(address).length !== 40) {
      throw new Error("Invalid address length");
    }

    return address;
  }

  const addressStr = bytesToHex(address);
  if (removeHexPrefix(addressStr).length !== 40) {
    throw new Error("Invalid address length");
  }

  return addressStr;
};

export { isAddress, getShardIdFromAddress, calculateAddress, refineAddress };
