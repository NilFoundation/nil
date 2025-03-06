import { createDomain } from "effector";
import { fetchCodeSnippet, setCodeSnippet } from "../../api/code";
import type { App } from "../../types";
import { tutorialWithStageRoute } from "../routing/routes/tutorialRoute";

export const codeDomain = createDomain("code");
export const isTutorialPage = tutorialWithStageRoute.$isOpened;

export const $code = codeDomain.createStore<string>("");
export const changeCode = codeDomain.createEvent<string>();
export const compile = codeDomain.createEvent();
export const $solidityVersion = codeDomain.createStore("v0.8.26+commit.8a97fa7a");
export const $availableSolidityVersions = codeDomain.createStore([
  "v0.8.28+commit.7893614a",
  "v0.8.27+commit.40a35a09",
  "v0.8.26+commit.8a97fa7a",
  "v0.8.25+commit.b61c2a91",
  "v0.8.24+commit.e11b9ed9",
]);

export const changeSolidityVersion = codeDomain.createEvent<string>();

export const $error = codeDomain.createStore<
  {
    message: string;
    line: number;
  }[]
>([]);
export const $warnings = codeDomain.createStore<
  {
    message: string;
    line: number;
  }[]
>([]);

export const compileCodeFx = codeDomain.createEffect<
  {
    code: string;
    version: string;
  },
  {
    apps: App[];
    warnings: {
      message: string;
      line: number;
    }[];
  }
>();

export const $codeSnippetHash = codeDomain.createStore<string | null>(null);
export const $shareCodeSnippetError = codeDomain.createStore<boolean>(false);

export const setCodeSnippetEvent = codeDomain.createEvent();
export const fetchCodeSnippetEvent = codeDomain.createEvent<string>();

export const setCodeSnippetFx = codeDomain.createEffect<string, string>();
export const fetchCodeSnippetFx = codeDomain.createEffect<string, string>();

export const changeIsTutorial = codeDomain.createEvent<boolean>();

setCodeSnippetFx.use((code) => {
  return setCodeSnippet(code);
});

fetchCodeSnippetFx.use((hash) => {
  return fetchCodeSnippet(hash);
});

export const loadedPlaygroundPage = codeDomain.createEvent();

export const loadedTutorialPage = codeDomain.createEvent();

export const сlickOnLogButton = codeDomain.createEvent();

export const сlickOnContractsButton = codeDomain.createEvent();

export const сlickOnBackButton = codeDomain.createEvent();

export const $recentProjects = codeDomain.createStore<Record<string, string>>({});

export const updateRecentProjects = codeDomain.createEvent();

export const triggerTutorialCheck = codeDomain.createEvent();
