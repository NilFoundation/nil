import { combine, sample } from "effector";
import { $smartAccount } from "../account-connector/model";
import { loadedTutorialPage } from "../code/model";
import { deploySmartContractFx } from "../contracts/models/base";
import { tutorialWithStageRoute } from "../routing/routes/tutorialRoute";
import {
  $tutorialCheck,
  deployTutorialContract,
  fetchTutorialCheckEvent,
  fetchTutorialCheckFx,
  runTutorialCheck,
  runTutorialCheckFx,
} from "./model";

$tutorialCheck.on(fetchTutorialCheckFx.doneData, (_, tutorialCheck) => tutorialCheck);

sample({
  clock: runTutorialCheck,
  source: $tutorialCheck,
  fn: (tutorialCheck) => tutorialCheck,
  target: runTutorialCheckFx,
});

sample({
  clock: loadedTutorialPage,
  source: tutorialWithStageRoute.$params,
  fn: (params) => Number(params.stage),
  filter: (stage) => stage !== undefined,
  target: fetchTutorialCheckFx,
});

sample({
  clock: fetchTutorialCheckEvent,
  source: fetchTutorialCheckEvent,
  fn: (tutorialCheck) => tutorialCheck.stage,
  target: fetchTutorialCheckFx,
});

sample({
  source: combine($smartAccount, (smartAccount) => ({ smartAccount })),
  filter: combine($smartAccount, (smartAccount) => !!smartAccount),
  clock: deployTutorialContract,
  fn: ({ smartAccount }, payload) => ({
    app: payload.app,
    args: payload.customArgs as unknown[],
    shardId: payload.customShardId,
    smartAccount: smartAccount!,
  }),
  target: deploySmartContractFx,
});
