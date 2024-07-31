import type { Hex } from "../../index.js";

/**
 * The type that contains the information necessary to deploy a contract.
 */
type IDeployData = {
  /**
   * The compiled contract bytecode.
   */
  bytecode: Hex;
  /**
   * The ID of the shard where the contract should be deployed.
   */
  shardId: number;
  /**
   * The public key of the contract ordering deployment.
   */
  pubkey?: Hex;
  /**
   * The sequence number of the deployment message.
   */
  seqno?: number;
};

export type { IDeployData };
