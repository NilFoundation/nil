import { createDomain } from "effector";
import type { App } from "../../types";
import loadTutorials from "./spec";

export enum TutorialLevel {
  Easy = "Easy",
  Medium = "Medium",
  Hard = "Hard",
  VeryHard = "Very hard",
}

export type Tutorial = {
  stage: number;
  text: string;
  contracts: string;
  icon: string;
  completionTime: string;
  level: TutorialLevel;
  title: string;
  description: string;
  urlSlug: string;
};

export const tutorialDomain = createDomain("tutorial");

export const $tutorial = tutorialDomain.createStore<Tutorial>({
  text: "",
  contracts: "",
  stage: 0,
  icon: "",
  completionTime: "",
  level: TutorialLevel.Easy,
  title: "",
  description: "",
  urlSlug: "",
});

export const $tutorials = tutorialDomain.createStore<Tutorial[]>([]);

export const $completedTutorials = tutorialDomain.createStore<number[]>([]);

export const $compiledTutorialContracts = tutorialDomain.createStore<App[]>([]);
export const $tutorialContracts = $tutorial.map((tutorial) => (tutorial ? tutorial.contracts : ""));

export const fetchTutorialEvent = tutorialDomain.createEvent<Tutorial>();
export const fetchAllTutorialsEvent = tutorialDomain.createEvent<Tutorial[]>();

export const fetchTutorialFx = tutorialDomain.createEffect<string, Tutorial, string>();
export const fetchAllTutorialsFx = tutorialDomain.createEffect<void, Tutorial[], string>();

export const notFoundTutorial = tutorialDomain.createEvent();

export const setCompletedTutorial = tutorialDomain.createEvent<number>();

fetchTutorialFx.use(async (urlSlug) => {
  const tutorials = await loadTutorials();
  const tutorial = tutorials.find((tutorial) => tutorial.urlSlug === urlSlug);
  if (!tutorial) {
    throw new Error(`Tutorial for URL ${urlSlug} not found`);
  }
  return tutorial;
});

fetchAllTutorialsFx.use(async () => {
  const tutorials = await loadTutorials();
  return tutorials;
});
