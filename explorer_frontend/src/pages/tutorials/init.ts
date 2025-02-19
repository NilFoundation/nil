import { persist } from "effector-storage/session";
import {
  compileCodeFx,
  сlickOnBackButton,
  сlickOnContractsButton,
  сlickOnLogButton,
} from "../../features/code/model";
import {
  $activeComponentTutorial,
  $tutorialChecksState,
  TutorialChecksStatus,
  TutorialLayoutComponent,
  setTutorialChecksState,
  сlickOnTutorialButton,
} from "./model";

$activeComponentTutorial.on(сlickOnLogButton, () => TutorialLayoutComponent.Logs);
$activeComponentTutorial.on(сlickOnContractsButton, () => TutorialLayoutComponent.Contracts);
$activeComponentTutorial.on(сlickOnBackButton, () => TutorialLayoutComponent.Code);
$activeComponentTutorial.on(сlickOnTutorialButton, () => TutorialLayoutComponent.TutorialText);
$tutorialChecksState.on(setTutorialChecksState, (_, payload) => {
  console.log("setTutorialChecksState", payload);
  return payload;
});
$tutorialChecksState.on(compileCodeFx.doneData, () => TutorialChecksStatus.Initialized);

persist({
  store: $activeComponentTutorial,
  key: "activeComponentTutorial",
});
