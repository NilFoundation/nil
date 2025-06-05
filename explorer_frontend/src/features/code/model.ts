import { createDomain } from "effector";
import { pending } from "patronum";
import { fetchProject, setProject } from "../../api/code";
import type { App, Project } from "./types";

export const codeDomain = createDomain("code");

export const $code = codeDomain.createStore<string>("");
export const $script = codeDomain.createStore<string>("");
export const changeCode = codeDomain.createEvent<string>();
export const changeScript = codeDomain.createEvent<string | null>();
export const compileCode = codeDomain.createEvent();
export const runScript = codeDomain.createEvent();
export const $solidityVersion = codeDomain.createStore("v0.8.26+commit.8a97fa7a");
export const $availableSolidityVersions = codeDomain.createStore([
  "v0.8.28+commit.7893614a",
  "v0.8.27+commit.40a35a09",
  "v0.8.26+commit.8a97fa7a",
  "v0.8.25+commit.b61c2a91",
  "v0.8.24+commit.e11b9ed9",
]);

export const changeSolidityVersion = codeDomain.createEvent<string>();

export const $codeError = codeDomain.createStore<
  {
    message: string;
    line: number;
  }[]
>([]);
export const $codeWarnings = codeDomain.createStore<
  {
    message: string;
    line: number;
  }[]
>([]);

export const $scriptErrors = codeDomain.createStore<
  {
    message: string;
    line: number;
  }[]
>([]);

export const $scriptWarnings = codeDomain.createStore<
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

export const runScriptFx = codeDomain.createEffect<
  {
    script: string;
    contracts: App[];
    rpcUrl: string;
  },
  {
    script: string;
    warnings: {
      message: string;
      line: number;
    }[];
  }
>();

export const $projectHash = codeDomain.createStore<string | null>(null);
export const $shareProjectError = codeDomain.createStore<boolean>(false);

export const setProjectEvent = codeDomain.createEvent();
export const fetchProjectEvent = codeDomain.createEvent<string>();

export const triggerCustomConsoleLogEvent = codeDomain.createEvent<string>();
export const triggerCustomConsoleWarnEvent = codeDomain.createEvent<string>();

export const setProjectFx = codeDomain.createEffect<Project, string>();
export const fetchProjectFx = codeDomain.createEffect<string, Project>();

setProjectFx.use(({ code, script }) => {
  return setProject({
    "Code.sol": code,
    "Script.ts": script || "",
  });
});

fetchProjectFx.use(async (hash) => {
  const res = await fetchProject(hash);
  return {
    code: res["Code.sol"] || "",
    script: res["Script.ts"] || "",
  };
});

export const loadedPlaygroundPage = codeDomain.createEvent();

export const loadedTutorialPage = codeDomain.createEvent();

export const clickOnLogButton = codeDomain.createEvent();

export const clickOnContractsButton = codeDomain.createEvent();

export const clickOnBackButton = codeDomain.createEvent();

export const $recentProjects = codeDomain.createStore<Record<string, string>>({});

export const updateRecentProjects = codeDomain.createEvent();

export const triggerTutorialCheck = codeDomain.createEvent();

export const $toolbarLoading = pending({ effects: [compileCodeFx, runScriptFx] });

export enum ProjectTab {
  code = "code",
  script = "script",
}

export const $projectTab = codeDomain.createStore<ProjectTab>(ProjectTab.code);

export const setProjectTab = codeDomain.createEvent<ProjectTab>();
