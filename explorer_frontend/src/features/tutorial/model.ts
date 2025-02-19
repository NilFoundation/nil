import { createDomain } from "effector";
import type { App } from "../../types";

export type Tutorial = {
  id: string;
  text: string;
  contracts: string;
  stage: number;
};

export const tutorialDomain = createDomain("tutorial");

export const $tutorial = tutorialDomain.createStore<Tutorial>({
  id: "",
  text: "",
  contracts: "",
  stage: 0,
});
export const $compiledTutorialContracts = tutorialDomain.createStore<App[]>([]);
export const $tutorialText = $tutorial.map((tutorial) => (tutorial ? tutorial.text : ""));
export const $tutorialContracts = $tutorial.map((tutorial) => (tutorial ? tutorial.contracts : ""));
export const $tutorialId = $tutorial.map((tutorial) => (tutorial ? tutorial.id : ""));
export const $tutorialStage = $tutorial.map((tutorial) => (tutorial ? tutorial.stage : 0));
