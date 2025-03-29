import { createEffect, createEvent } from "effector";
import type { TutorialChecksStatus } from "../../src/pages/tutorials/model";
import type { App } from "../../src/features/code/types";
import type { Hex, SmartAccountV1 } from "@nilfoundation/niljs";
import { deployContractFunction } from "../../src/features/contracts/models/base";

export const RPC_ENDPOINT = "http://127.0.0.1:8529";
export const SOLIDITY_COMPILER_VERSION = 'v0.8.28+commit.7893614a';

export const deploymentEffect = createEffect<{
  app: App;
  args: unknown[];
  shardId: number;
  smartAccount: SmartAccountV1;
},
  {
    address: Hex;
    app: Hex;
    name: string;
    deployedFrom?: Hex;
    txHash: Hex;
  }>(async ({ app, args, smartAccount, shardId }) => {
    return await deployContractFunction({ app, args, smartAccount, shardId });
  });

export const setTutorialChecksEvent = createEvent<TutorialChecksStatus>();
export const tutorialContractStepFailed = createEvent<string>();
export const tutorialContractStepPassed = createEvent<string>();
export const setCompletedTutorialEvent = createEvent<number>();
