import { version } from "../version.js";

/**
 * The interface for the parameters of the BaseError constructor.
 */
type IBaseErrorParameters = {
  /**
   * The flag that indicates if this error is operational.
   * This is useful to differentiate operational errors from programming errors.
   * It is recommended to always set this property to true when creating a custom error class.
   * @default true
   */
  isOperational?: boolean;
  /**
   * The error cause.
   */
  cause?: Error | BaseError;
  /**
   * The path to the documentation of this error.
   */
  docsPath?: string;
};

/**
 * The base class for custom errors.
 */
class BaseError extends Error {
  /**
   * The flag that indicates if this error is operational.
   * This is useful to differentiate operational errors from programming errors.
   * It is recommended to always set this property to true when creating a custom error class.
   * @public
   * @type {boolean}
   */
  public isOperational: boolean;
  /**
   * The error cause.
   *
   * @public
   * @type {?(Error | BaseError)}
   */
  public cause?: Error | BaseError;
  /**
   * The path to the documentation of this error.
   *
   * @public
   * @type {?string}
   */
  public docsPath?: string;
  /**
   * The version of the client.
   */
  public version?: string;

  /**
   * Creates an instance of BaseError.
   *
   * @constructor
   * @param {?string} [message] The error message.
   * @param {IBaseErrorParameters} [param0={}] The error params.
   * @param {boolean} [param0.isOperational=true] The flag that determines whether the error is operational.
   * @param {*} param0.cause The error cause.
   * @param {string} param0.docsPath The path to the documentation of this error.
   * @param {string} param0.version The version of the client.
   */
  constructor(
    message?: string,
    { isOperational = true, cause, docsPath }: IBaseErrorParameters = {},
  ) {
    super();
    this.name = this.constructor.name;
    this.isOperational = isOperational;
    this.cause = cause;
    this.docsPath = docsPath;
    this.version = version;

    this.message = `${message ?? "An error occurred"}
      Name: ${this.name}`;

    if (docsPath) {
      this.message = `${this.message}
      Docs: see \${this.docsPath}`;
    }

    if (version) {
      this.message = `${this.message}
      Version of the client: ${this.version}`;
    }

    // This line is needed to make the instanceof operator work correctly with custom errors in TypeScript
    Object.setPrototypeOf(this, BaseError.prototype);
  }
}

export { BaseError, type IBaseErrorParameters };
