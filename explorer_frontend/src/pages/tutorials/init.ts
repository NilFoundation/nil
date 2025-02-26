import { sample } from "effector";
import { persist } from "effector-storage/session";
import {
  compileCodeFx,
  userClickOnBackButton,
  userClickOnContractsButton,
  userClickOnLogButton,
  userClickOnTutorialButton,
} from "../../features/code/model";
import {
  $activeComponentTutorial,
  $tutorialChecksState,
  TutorialLayoutComponent,
  setTutorialChecksState,
} from "./model";

$activeComponentTutorial.on(userClickOnLogButton, (_) => TutorialLayoutComponent.Logs);
$activeComponentTutorial.on(userClickOnContractsButton, (_) => TutorialLayoutComponent.Contracts);
$activeComponentTutorial.on(userClickOnBackButton, (_) => TutorialLayoutComponent.Code);
$activeComponentTutorial.on(userClickOnTutorialButton, (_) => TutorialLayoutComponent.TutorialText);
$tutorialChecksState.on(setTutorialChecksState, (_) => true);

sample({
  source: $tutorialChecksState,
  clock: compileCodeFx.doneData,
  target: setTutorialChecksState,
});

persist({
  store: $activeComponentTutorial,
  key: "activeComponentTutorial",
});
