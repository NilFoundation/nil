import type { Hex, SmartAccountV1 } from "@nilfoundation/niljs";
import type { Effect, Event } from "effector";
import type { TutorialChecksStatus } from "../../pages/tutorials/model";
import type { App } from "../code/types";
import {} from "./model";

export interface CheckProps {
  rpcUrl: string;
  deploymentEffect: Effect<
    {
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
    }
  >;
  setTutorialChecksEvent: Event<TutorialChecksStatus>;
  tutorialContractStepFailed: Event<string>;
  tutorialContractStepPassed: Event<string>;
  contracts: App[];
  setCompletedTutorialEvent: Event<number>;
}
