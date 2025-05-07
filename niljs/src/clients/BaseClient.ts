import { assertIsValidShardId } from "../utils/assert.js";
import { RPCClient } from "./RPCCleint.js";
import type { IClientBaseConfig } from "./types/Configs.js";

/**
 * BaseClient is the base class for any client tasked with interacting with =nil;
 * @class BaseClient
 * @typedef {BaseClient}
 */
class BaseClient extends RPCClient {
  /**
   * The ID of the shard with which the client needs to interact.
   * The shard with this ID will be used in every call made by the client.
   * @public
   * @type {number | undefined}
   */
  public shardId?: number;

  /**
   * Creates an instance of BaseClient.
   * @constructor
   * @param {IClientBaseConfig} config The config to be used in the client. It contains the transport and the shard ID. See {@link IClientBaseConfig}.
   */
  constructor(config: IClientBaseConfig) {
    super(config);
    this.shardId = config.shardId;
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
