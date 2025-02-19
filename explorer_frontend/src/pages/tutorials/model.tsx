import { createEvent, createStore } from "effector";

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
}

export const $activeComponentTutorial = createStore<TutorialLayoutComponent>(
  TutorialLayoutComponent.Code,
);

export const setActiveComponentTutorial = createEvent<TutorialLayoutComponent>();

export const $tutorialChecksState = createStore<TutorialChecksStatus>(
  TutorialChecksStatus.NotInitialized,
);

export const setTutorialChecksState = createEvent<TutorialChecksStatus>();

export const сlickOnTutorialButton = createEvent();
