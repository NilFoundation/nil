import type { BlockTag } from "../types/Block.js";
import type { Hex } from "../types/Hex.js";
import { BaseError, type IBaseErrorParameters } from "./BaseError.js";

/**
 * The interface for the parameters of the block errors constructor.
 */
type BlockErrorParameters = {
  blockNumberOrHash: Hex | BlockTag;
} & IBaseErrorParameters;

/**
 * The error class for 'block not found' errors.
 * This error is thrown when the requested block is not found.
 */
class BlockNotFoundError extends BaseError {
  constructor({ blockNumberOrHash, ...rest }: BlockErrorParameters) {
    super(`Block ${blockNumberOrHash} not found.`, {
      name: "BlockNotFoundError",
      ...rest,
    });
  }
}

export { BlockNotFoundError };
