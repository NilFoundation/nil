import { BaseError, type IBaseErrorParameters } from "./BaseError.js";

/**
 * The interface for the parameters of the {@link BlockNotFoundError} constructor.
 */
type IntegerOutOfRangeErrorParameters = {
  max?: number | bigint;
  min: number | bigint;
  value: number | bigint;
} & IBaseErrorParameters;

/**
 * The error class for 'integer out of range' errors.
 * This error is thrown when the requested integer is out of range.
 */
class IntegerOutOfRangeError extends BaseError {
  /**
   * Creates an instance of IntegerOutOfRangeError.
   *
   * @constructor
   * @param {IntegerOutOfRangeErrorParameters} param0 The error params.
   * @param {*} param0.max The maximum value.
   * @param {string} param0.min The minimum value.
   */
  constructor({ max, min, value, ...rest }: IntegerOutOfRangeErrorParameters) {
    super(
      `Number "${value}" is not in safe integer range ${max ? `(${min} to ${max})` : `(above ${min})`}`,
      { ...rest },
    );
  }
}

export { IntegerOutOfRangeError };
