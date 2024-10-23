import type { BlockTag, Hex } from "../index.js";
import { BaseError, type IBaseErrorParameters } from "./BaseError.js";

/**
 * The interface for the parameters of the {@link BlockNotFoundError} constructor.
 */
type BlockNotFoundErrorParameters = {
  blockNumberOrHash: Hex | BlockTag;
} & IBaseErrorParameters;

/**
 * The error class for 'block not found' errors.
 * This error is thrown when the requested block is not found.
 */
class BlockNotFoundError extends BaseError {
  /**
   * Creates an instance of BlockNotFoundError.
   *
   * @constructor
   * @param {BlockNotFoundErrorParameters} param0 The error params.
   * @param {*} param0.blockNumberOrHash The block number or hash.
   * @param {*} param0....rest The remaining error params, see {@link IBaseErrorParameters}.
   */
  constructor({ blockNumberOrHash, ...rest }: BlockNotFoundErrorParameters) {
    super(`Block not found: ${blockNumberOrHash}`, { ...rest });
  }
}

export { BlockNotFoundError };
