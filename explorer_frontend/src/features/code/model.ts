import { createDomain } from "effector";
import { fetchCodeSnippet, setCodeSnippet } from "../../api/code";
import { fetchAllTutorials, fetchTutorial } from "../../api/tutorial";
import type { App } from "../../types";
import type { Tutorial } from "../../types";
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

export const fetchTutorialEvent = codeDomain.createEvent<Tutorial>();
export const fetchAllTutorialsEvent = codeDomain.createEvent<Tutorial[]>();

export const setCodeSnippetFx = codeDomain.createEffect<string, string>();
export const fetchCodeSnippetFx = codeDomain.createEffect<string, string>();

export const fetchTutorialFx = codeDomain.createEffect<string, Tutorial, string>();
export const fetchAllTutorialsFx = codeDomain.createEffect<void, Tutorial[], string>();

export const changeIsTutorial = codeDomain.createEvent<boolean>();

setCodeSnippetFx.use((code) => {
  return setCodeSnippet(code);
});

fetchCodeSnippetFx.use((hash) => {
  return fetchCodeSnippet(hash);
});

fetchTutorialFx.use((stage) => {
  return fetchTutorial(stage);
});

fetchAllTutorialsFx.use(() => {
  return fetchAllTutorials();
});

export const loadedPlaygroundPage = codeDomain.createEvent<boolean>();

export const loadedTutorialPage = codeDomain.createEvent<boolean>();

export const userClickOnLogButton = codeDomain.createEvent();

export const userClickOnContractsButton = codeDomain.createEvent();

export const userClickOnBackButton = codeDomain.createEvent();

export const userClickOnTutorialButton = codeDomain.createEvent();

export const $recentProjects = codeDomain.createStore<Record<string, string>>({});

export const updateRecentProjects = codeDomain.createEvent();
