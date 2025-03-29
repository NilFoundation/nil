import { createEvent, createStore } from "effector";
import type { Tutorial } from "../../features/tutorial/model";

export enum TutorialChecksStatus {
  NotInitialized = "0",
  Initialized = "1",
  Successful = "2",
  Failed = "3",
}

export enum TutorialLayoutComponent {
  Code = "0",
  Contracts = "1",
  Logs = "2",
  TutorialText = "3",
  Tutorials = "4",
}

export const $activeComponentTutorial = createStore<TutorialLayoutComponent>(
  TutorialLayoutComponent.Code,
);

export const setActiveComponentTutorial = createEvent<TutorialLayoutComponent>();

export const $tutorialChecksState = createStore<TutorialChecksStatus>(
  TutorialChecksStatus.NotInitialized,
);

export const setTutorialChecksState = createEvent<TutorialChecksStatus>();

export const openTutorialText = createEvent();

export const $selectedTutorial = createStore<Tutorial | null>(null);

export const setSelectedTutorial = createEvent<Tutorial | null>();

export const clickOnTutorialsBackButton = createEvent();

export const changeActiveTab = createEvent<string>();

export const $activeTab = createStore("0").on(changeActiveTab, (_, newTab) => newTab);
