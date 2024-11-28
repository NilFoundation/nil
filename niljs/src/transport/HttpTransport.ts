import type { Client as RPCClient } from "@open-rpc/client-js";
import type { RequestArguments } from "@open-rpc/client-js/build/ClientInterface.js";
import { createRPCClient } from "../rpc/rpcClient.js";
import type { IHttpTransportConfig } from "./types/IHttpTransportConfig.js";
import type { ITransport } from "./types/ITransport.js";

/**
 * HttpTransport represents the HTTP transport for connecting to the network.
 *
 * @class HttpTransport
 * @typedef {HttpTransport}
 * @implements {ITransport}
 */
class HttpTransport implements ITransport {
  /**
   * The RPC client for the transport.
   *
   * @private
   * @type {RPCClient}
   */
  private rpcClient: RPCClient;
  /**
   * The request timeout.
   *
   * @private
   * @type {number}
   */
  private timeout: number;

  /**
   * Creates an instance of HttpTransport.
   *
   * @constructor
   * @param {IHttpTransportConfig} config The transport config. See {@link IHttpTransportConfig}.
   */
  constructor({ endpoint, signal, timeout = 20000 }: IHttpTransportConfig) {
    this.rpcClient = createRPCClient(endpoint, { signal });
    this.timeout = timeout;
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
    return this.rpcClient.request(requestObject, this.timeout);
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
  public closeConnection(): void {
    this.rpcClient.close();
  }
}

export { HttpTransport };
