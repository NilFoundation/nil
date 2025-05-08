import type { JSONRPCClient, JSONRPCRequest } from "json-rpc-2.0";

/**
 * The transport interface.
 */
abstract class ITransport {
  /**
   * Sends a request and passes the reposnse to the JSON-RPC client.
   * @param request - The request object.
   * @returns The response.
   */
  abstract request(request: JSONRPCRequest, client: JSONRPCClient): Promise<void>;
}

export { ITransport };
