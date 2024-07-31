import type { Abi } from "abitype";
import type { Hex } from "./Hex.js";

/**
 * IDeployData is a data structure that contains information to deploy a contract.
 */
type deployDataBase = {
  bytecode: Uint8Array | Hex;
  salt: Uint8Array | bigint;
  shard: number;
};

type deployDataWithArgs = deployDataBase & {
  args: unknown[];
  abi: Abi;
};

type deployDataWithoutArgs = deployDataBase & {
  args?: undefined;
  abi?: undefined;
};

type IDeployData = deployDataWithArgs | deployDataWithoutArgs;

export type { IDeployData };
