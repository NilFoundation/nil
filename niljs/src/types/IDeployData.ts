import type { Abi } from "abitype";
import type { Hex } from "./Hex.js";

/**
 * IDeployData is a data structure that contains information to deploy a contract.
 */
type DeployDataBase = {
  bytecode: Uint8Array | Hex;
  salt: Uint8Array | bigint;
  shard: number;
  chainId?: number;
  feeCredit?: bigint;
};

type DeployDataWithArgs = DeployDataBase & {
  args: unknown[];
  abi: Abi;
};

type DeployDataWithoutArgs = DeployDataBase & {
  args?: undefined;
  abi?: undefined;
};

type IDeployData = DeployDataWithArgs | DeployDataWithoutArgs;

export type { IDeployData };
