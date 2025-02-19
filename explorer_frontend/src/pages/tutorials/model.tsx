import { createEvent, createStore } from "effector";

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

export const $tutorialChecksState = createStore<boolean>(false);

export const setTutorialChecksState = createEvent<boolean>();
