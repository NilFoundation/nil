import { JSONRPCClient, type JSONRPCRequest } from "json-rpc-2.0";
import type { ITransport } from "../transport/types/ITransport.js";
import type { IRPCClientConfig } from "./types/Configs.js";
import type { JSONRPCRequestArguments } from "./types/RPC.js";

/**
 * RPCClient is used for creating RPC requests.
 * @class RPCClient
 */
class RPCClient {
  /**
   * The JSON-RPC client to be used in the client. See {@link JSONRPCClient}.
   * It is used as an entity that creates requests and parses responses.
   * Request logic can be implemented in the transport layer.
   *
   * @private
   * @type {JSONRPCClient}
   */
  private requester: JSONRPCClient;

  /**
   * The ITransport to be used in the client. See {@link ITransport}.
   *
   * @readonly
   * @type {ITransport}
   */
  readonly transport: ITransport;

  /**
   * Creates an instance of RPCClient.
   * @constructor
   * @param {IRPCClientConfig} config The config to be used in the client. It contains the transport and the shard ID. See {@link IClientBaseConfig}.
   */
  constructor(config: IRPCClientConfig) {
    this.transport = config.transport;
    this.requester = new JSONRPCClient((req: JSONRPCRequest) =>
      this.transport.request(req, this.requester),
    );
  }

  /**
   * Sends a request.
   * @param requestObject The request object. It contains the request method and parameters.
   * @returns The response.
   */
  protected async request<T>(requestObject: JSONRPCRequestArguments): Promise<T> {
    return await this.requester.request(requestObject.method, requestObject.params);
  }
}

export { RPCClient };
