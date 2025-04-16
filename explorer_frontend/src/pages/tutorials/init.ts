import { sample } from "effector";
import { persist } from "effector-storage/session";
import {
  clickOnBackButton,
  clickOnContractsButton,
  clickOnLogButton,
  compileCodeFx,
  loadedTutorialPage,
} from "../../features/code/model";
import { tutorialWithUrlStringRoute } from "../../features/routing/routes/tutorialRoute";
import {
  $activeComponentTutorial,
  $selectedTutorial,
  $tutorialChecksState,
  TutorialChecksStatus,
  TutorialLayoutComponent,
  clickOnTutorialsBackButton,
  openTutorialText,
  setActiveComponentTutorial,
  setSelectedTutorial,
  setTutorialChecksState,
} from "./model";

$activeComponentTutorial.on(setActiveComponentTutorial, (_, payload) => payload);
$activeComponentTutorial.on(clickOnLogButton, () => TutorialLayoutComponent.Logs);
$activeComponentTutorial.on(clickOnContractsButton, () => TutorialLayoutComponent.Contracts);
$activeComponentTutorial.on(clickOnBackButton, () => TutorialLayoutComponent.Code);
$activeComponentTutorial.on(openTutorialText, () => TutorialLayoutComponent.TutorialText);
$activeComponentTutorial.on(clickOnTutorialsBackButton, () => TutorialLayoutComponent.Tutorials);
$tutorialChecksState.on(setTutorialChecksState, (_, payload) => payload);
$tutorialChecksState.on(compileCodeFx.doneData, () => TutorialChecksStatus.Initialized);

sample({
  clock: setSelectedTutorial,
  target: $selectedTutorial,
});

sample({
  clock: clickOnTutorialsBackButton,
  fn: () => null,
  target: setSelectedTutorial,
});

sample({
  clock: [loadedTutorialPage, tutorialWithUrlStringRoute.$params],
  fn: () => TutorialChecksStatus.NotInitialized,
  target: setTutorialChecksState,
});
$tutorialChecksState.on(compileCodeFx.doneData, () => TutorialChecksStatus.Initialized);

persist({
  store: $activeComponentTutorial,
  key: "activeComponentTutorial",
});
