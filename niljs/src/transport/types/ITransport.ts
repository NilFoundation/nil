import type { RequestArguments } from "@open-rpc/client-js/build/ClientInterface.js";

/**
 * The transport interface.
 */
abstract class ITransport {
  /**
   * Sends a request.
   * @param requestObject - The request object. It contains the method and parameters.
   * @returns The response.
   */
  abstract request<T>(requestObject: RequestArguments): Promise<T>;

  /**
   * Connects to the network.
   */
  abstract connect(): void;

  /**
   * Closes the connection to the network.
   */
  abstract closeConnection(): void;
}

export { ITransport };
