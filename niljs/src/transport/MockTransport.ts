import type { RequestArguments } from "@open-rpc/client-js/build/ClientInterface.js";
import type { ITransport } from "./types/ITransport.js";

/**
 * The MockTransport is a transport class for testing purposes.
 *
 * @class MockTransport
 * @typedef {MockTransport}
 * @implements {ITransport}
 */
class MockTransport implements ITransport {
  private handler: (args: RequestArguments) => unknown;
  /**
   * Creates an instance of MockTransport.
   *
   * @constructor
   * @param {function} handler The testing handler.
   */
  constructor(handler: (args: RequestArguments) => unknown) {
    this.handler = handler;
  }

  /**
   * Sends a request to the network.
   *
   * @public
   * @async
   * @template T
   * @param {RequestArguments} requestObject The request object.
   * @returns {Promise<T>} The response.
   */
  public async request<T>(requestObject: RequestArguments): Promise<T> {
    return this.handler(requestObject) as T;
  }

  /**
   * Connects to the network.
   *
   * @public
   */
  public connect(): void {
    //
  }

  /**
   * Closes the connection to the network.
   *
   * @public
   */
  public closeConnection(): void {}
}

export { MockTransport };
