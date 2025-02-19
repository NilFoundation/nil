import { createDomain } from "effector";
import type { App } from "../../types";
import { spec } from "./spec";

export type TutorialCheck = {
  stage: number;
  check: () => Promise<void>;
};

export const tutorialCheckDomain = createDomain("tutorial-check");

export const $tutorialCheck = tutorialCheckDomain.createStore<TutorialCheck>({
  stage: 0,
  check: async () => {},
});

export const deployTutorialContract = tutorialCheckDomain.createEvent<{
  app: App;
  customArgs: Record<string, string | boolean>[];
  customShardId: number;
}>();

export const tutorialContractStepPassedEvent = tutorialCheckDomain.createEvent<string>();

export const tutorialContractStepFailedEvent = tutorialCheckDomain.createEvent<string>();

export const fetchTutorialCheckEvent = tutorialCheckDomain.createEvent<TutorialCheck>();

export const fetchTutorialCheckFx = tutorialCheckDomain.createEffect<number, TutorialCheck>();

export const runTutorialCheck = tutorialCheckDomain.createEvent();

export const runTutorialCheckFx = tutorialCheckDomain.createEffect(
  async (tutorialCheck: TutorialCheck) => {
    return await tutorialCheck.check();
  },
);

fetchTutorialCheckFx.use(async (stage) => {
  const tutorialCheck = spec.find((check) => check.stage === stage)!;
  return tutorialCheck;
});
