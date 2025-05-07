import type { JSONRPCClient, JSONRPCRequest, JSONRPCSuccessResponse } from "json-rpc-2.0";
import type { ITransport } from "./types/ITransport.js";

/**
 * The partial type representing the request arguments, which we typically use in cloients.
 */
type RequestArguments = {
  method: string;
  params: unknown[];
};

/**
 * The MockTransport is a transport class for testing purposes.
 *
 * @class MockTransport
 * @typedef {MockTransport}
 * @implements {ITransport}
 */
class MockTransport implements ITransport {
  /**
   * The handler to be used in the transport.
   */
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
  public async request(requestObject: JSONRPCRequest, client: JSONRPCClient) {
    /**
     * We want to test method and params only.
     */
    const result = this.handler({
      method: requestObject.method,
      params: requestObject.params,
    });

    console.log("MockTransport: request", requestObject);

    const simulatedResponse: JSONRPCSuccessResponse = {
      jsonrpc: "2.0",
      result,
      id: requestObject.id ?? 0,
    };

    console.log("MockTransport: response", simulatedResponse);

    client.receive(simulatedResponse);
  }
}

export { MockTransport };
