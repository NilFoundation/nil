import type { RequestArguments } from "@open-rpc/client-js/build/ClientInterface.js";
import type { ITransport } from "../transport/index.js";
import { assertIsValidShardId } from "../utils/assert.js";
import type { IClientBaseConfig } from "./types/Configs.js";

/**
 * BaseClient is the base class for any client tasked with interacting with =nil;
 * @class BaseClient
 * @typedef {BaseClient}
 */
class BaseClient {
  /**
   * The ITransport to be used in the client. See {@link ITransport}.
   *
   * @protected
   * @type {ITransport}
   */
  protected transport: ITransport;

  /**
   * The ID of the shard with which the client needs to interact.
   * The shard with this ID will be used in every call made by the client.
   * @protected
   * @type {number | undefined}
   */
  protected shardId?: number;

  /**
   * Creates an instance of BaseClient.
   * @constructor
   * @param {IClientBaseConfig} config The config to be used in the client. It contains the transport and the shard ID. See {@link IClientBaseConfig}.
   */
  constructor(config: IClientBaseConfig) {
    this.transport = config.transport;
    this.shardId = config.shardId;
  }

  /**
   * Sends a request.
   * @param requestObject The request object. It contains the request method and parameters.
   * @returns The response.
   */
  protected async request<T>(requestObject: RequestArguments): Promise<T> {
    return this.transport.request(requestObject);
  }

  /**
   * Returns the shard ID.
   * @returns The shard ID.
   */
  public getShardId() {
    return this.shardId;
  }

  /**
   * Sets the shard ID.
   * @param shardId The shard ID.
   * @throws Will throw an error if the provided shard ID is invalid.
   * @example
   * client.setShardId(1);
   */
  public setShardId(shardId: number): void {
    assertIsValidShardId(shardId);

    this.shardId = shardId;
  }
}

export { BaseClient };
