import invariant from "tiny-invariant";
import { masterShardId } from "../clients/constants.js";
import type { IDeployData } from "../clients/types/IDeployData.js";
import { type Hex, InvalidShardIdError } from "../index.js";
import type { IPrivateKey } from "../signers/index.js";
import type { Block } from "../types/Block.js";
import { isAddress } from "./address.js";
import { isValidBlock } from "./block.js";
import { isHexString } from "./hex.js";

/**
 * Checks if the value is a string.
 * @throws Will throw an error if the value is not a hex string.
 * @param value - The value to check.
 * @param message - The message to throw if the value is not a hex string.
 */
const assertIsHexString = (value: Hex, message?: string): void => {
  invariant(isHexString(value), message ?? `Expected a hex string but got ${value}`);
};

/**
 * Checks if the value is a buffer.
 * @throws Will throw an error if value is not a buffer.
 * @param value - The value to check.
 * @param message - The message to throw if the value is not a buffer.
 */
const assertIsBuffer = (value: Uint8Array, message?: string): void => {
  invariant(value instanceof Uint8Array, message ?? `Expected a buffer but got ${value}`);
};

/**
 * Checks if provided private key is valid. If the value is a hex string with length 32 nothing is returned.
 * @throws Will throw an error if provided private key is invalid.
 * @param privateKey - The private key to check.
 * @param message - The message to throw if the private key is invalid.
 */
const assertIsValidPrivateKey = (privateKey: IPrivateKey, message?: string): void => {
  invariant(
    isHexString(privateKey) && privateKey.length === 32 * 2 + 2,
    message ?? `Expected a valid private key, but got ${privateKey}`,
  );
};

/**
 * Checks if the data to deploy contract is valid. If the data is valid, it returns nothing.
 * @throws Will throw an error if the value is not a valid data to deploy contract.
 * @param deployData - The data to validate.
 * @param message - The message to throw if the data is invalid.
 */
const assertIsValidDeployData = (deployContractData: IDeployData, message?: string) => {
  const { seqno, pubkey, shardId } = deployContractData;

  if (seqno !== undefined) {
    invariant(seqno >= 0, message ?? `Expected a valid 'seqno' but got ${seqno}`);
  }

  if (pubkey !== undefined) {
    invariant(typeof pubkey === "string", message ?? `Expected a valid 'pubkey' but got ${pubkey}`);
  }

  assertIsValidShardId(shardId);
};

/**
 * Checks if the address is valid. If the address is valid, it returns nothing.
 * @param address - The address to check.
 * @param message - The message to throw if the address is invalid.
 */
const assertIsAddress = (address: string, message?: string): void => {
  invariant(isAddress(address), message ?? `Expected a valid address but got ${address}`);
};

/**
 * Checks if the block is valid. If the block is valid, it returns nothing.
 * @param block - The block to check.
 * @param message - The message to throw if the block is invalid.
 */
const assertIsValidBlock = (block: Block, message?: string): void => {
  invariant(isValidBlock(block), message ?? `Expected a valid block but got ${block}`);
};

/**
 * Checks if the shard id is valid. If the shard id is valid, it returns nothing.
 * @param shardId - The shard id to check.
 */
const assertIsValidShardId = (shardId?: number): void => {
  const isValid =
    typeof shardId === "number" &&
    Number.isInteger(shardId) &&
    shardId >= 0 &&
    shardId < 2 ** 16 &&
    shardId !== masterShardId;

  if (!isValid) {
    throw new InvalidShardIdError({ shardId });
  }
};

export {
  assertIsBuffer,
  assertIsHexString,
  assertIsValidPrivateKey,
  assertIsAddress,
  assertIsValidBlock,
  assertIsValidShardId,
  assertIsValidDeployData,
};
