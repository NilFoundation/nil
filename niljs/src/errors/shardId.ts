import { BaseError, type IBaseErrorParameters } from "./BaseError.js";

/**
 * The interface for the parameters of the {@link InvalidShardIdError} constructor.
 */
type InvalidShardIdErrorParameters = {
  shardId?: number;
} & IBaseErrorParameters;

/**
 * The error class for invalid shard ID.
 * This error is thrown when the provided shard ID is invalid.
 */
class InvalidShardIdError extends BaseError {
  /**
   * Creates an instance of InvalidShardIdError.
   *
   * @constructor
   * @param {InvalidShardIdErrorParameters} param0 The error params.
   * @param {*} param0.shardId The invalid shard ID.
   * @param {*} param0....rest The remaining error params, see {@link IBaseErrorParameters}.
   */
  constructor({ shardId, ...rest }: InvalidShardIdErrorParameters) {
    super(`Expected a valid shardId but got: ${shardId}`, {
      name: "InvalidShardIdError",
      ...rest,
    });
  }
}

export { InvalidShardIdError };
