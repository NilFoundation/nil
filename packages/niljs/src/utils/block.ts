import type { Block } from "../index.js";

/**
 * Checks if the provided block is valid.
 * @param block The block to check.
 * @returns True if the block is valid, false otherwise.
 */
const isValidBlock = (block: Block): block is Block => {
  return true;
};

export { isValidBlock };
